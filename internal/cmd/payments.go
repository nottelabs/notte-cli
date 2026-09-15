package cmd

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

func init() { rootCmd.AddCommand(newPaymentCommand()) }

func newPaymentCommand() *cobra.Command {
	group := &cobra.Command{Use: "payment", Short: "Request a temporary card for a browser session"}
	var body api.SessionPaymentRequest
	var sessionID, key, amount string
	request := &cobra.Command{Use: "request", Short: "Request spending and receive wallet connection or approval instructions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if _, err := uuid.Parse(sessionID); err != nil {
			if err := ValidateSessionID(sessionID)(); err != nil {
				return err
			}
		}
		if len(amount) > 24 || !regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`).MatchString(amount) {
			return fmt.Errorf("--amount must be a positive decimal in currency units (e.g. 100.91)")
		}
		parsed, ok := new(big.Rat).SetString(amount)
		if !ok || parsed.Sign() <= 0 || parsed.Cmp(big.NewRat(50000, 1)) > 0 {
			return fmt.Errorf("--amount must be positive and within the currency's spending limit")
		}
		// Preserve the user's decimal text; the API validates currency precision
		// and converts to Link minor units without floating-point arithmetic.
		body.Amount = amount
		if !regexp.MustCompile(`^[a-z]{3}$`).MatchString(body.Currency) {
			return fmt.Errorf("--currency must be three lowercase letters")
		}
		u, err := url.Parse(body.MerchantURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || utf8.RuneCountInString(body.MerchantURL) > 2048 {
			return fmt.Errorf("--merchant-url must be an HTTPS URL without credentials (maximum 2048 characters)")
		}
		if strings.TrimSpace(body.MerchantName) == "" || utf8.RuneCountInString(body.MerchantName) > 200 {
			return fmt.Errorf("--merchant-name must contain 1 to 200 characters")
		}
		if n := utf8.RuneCountInString(body.Description); n < 100 || n > 4000 {
			return fmt.Errorf("--description must contain 100 to 4000 characters")
		}
		if body.Mode != "test" && body.Mode != "live" {
			return fmt.Errorf("--mode must be test or live")
		}
		if key == "" {
			key, err = api.GenerateIdempotencyKey()
			if err != nil {
				return err
			}
		}
		if len(key) > 200 {
			return fmt.Errorf("--idempotency-key must be at most 200 characters")
		}
		// stderr keeps JSON stdout a single document and lets users replay an uncertain request.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Idempotency key: %s\n", key)
		client, err := GetClient()
		if err != nil {
			return err
		}
		result, err := fetchPayment(cmd.Context(), client, sessionID, "", key, &body)
		if err != nil {
			return err
		}
		return GetFormatter().Print(result)
	}}
	f := request.Flags()
	f.StringVar(&sessionID, "session-id", "", "Existing browser session ID")
	f.StringVar(&amount, "amount", "", "Amount in currency units (100.91 = USD 100.91)")
	f.StringVar(&body.Currency, "currency", "usd", "Three-letter lowercase currency")
	f.StringVar(&body.MerchantURL, "merchant-url", "", "HTTPS merchant URL")
	f.StringVar(&body.MerchantName, "merchant-name", "", "Merchant name")
	f.StringVar(&body.Description, "description", "", "Purchase description (100 to 4000 characters)")
	f.StringVar(&body.Mode, "mode", "test", "Payment mode: test or live (requires backend enablement)")
	f.StringVar(&key, "idempotency-key", "", "Reuse this key with the same request to recover a previous attempt")
	for _, name := range []string{"session-id", "amount", "merchant-url", "merchant-name", "description"} {
		_ = request.MarkFlagRequired(name)
	}
	group.AddCommand(request)
	status := &cobra.Command{Use: "status PAYMENT_ID", Short: "Retrieve payment status and current approval instructions", Args: paymentIDArg, RunE: func(cmd *cobra.Command, args []string) error {
		client, err := GetClient()
		if err != nil {
			return err
		}
		result, err := fetchPayment(cmd.Context(), client, "", args[0], "", nil)
		if err != nil {
			return err
		}
		return GetFormatter().Print(result)
	}}
	var timeout time.Duration
	wait := &cobra.Command{Use: "wait PAYMENT_ID", Short: "Wait until credentials are ready or the payment fails", Long: "Wait until the temporary card is installed in the session vault. Ready does not mean a purchase succeeded. Connection, approval, and verification instructions appear on stderr. Exiting does not cancel provisioning.", Args: paymentIDArg, RunE: func(cmd *cobra.Command, args []string) error {
		if timeout < 0 {
			return fmt.Errorf("--wait-timeout must not be negative")
		}
		client, err := GetClient()
		if err != nil {
			return err
		}
		ctx := cmd.Context()
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return waitPayment(cmd, ctx, client, args[0], 2*time.Second)
	}}
	wait.Flags().DurationVar(&timeout, "wait-timeout", 0, "Maximum time to wait (e.g. 10m); zero waits until completion")
	group.AddCommand(status, wait)
	return group
}

func paymentIDArg(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return err
	}
	if _, err := uuid.Parse(args[0]); err != nil {
		return fmt.Errorf("payment ID must be a UUID")
	}
	return nil
}

func fetchPayment(ctx context.Context, client *api.NotteClient, sessionID, paymentID, key string, body *api.SessionPaymentRequest) (*api.PaymentStatus, error) {
	ctx, cancel := GetContextWithTimeout(ctx)
	defer cancel()
	result, resp, raw, err := client.Payment(ctx, sessionID, paymentID, key, body)
	if err != nil {
		return nil, fmt.Errorf("payment request failed: %w", err)
	}
	if err := HandleAPIResponse(resp, raw); err != nil {
		return nil, err
	}
	if result == nil || result.ID == "" || result.Status == "" {
		return nil, fmt.Errorf("invalid payment response: missing ID or status")
	}
	return result, nil
}

func waitPayment(cmd *cobra.Command, ctx context.Context, client *api.NotteClient, id string, interval time.Duration) error {
	lastInstructions := ""
	for {
		result, err := fetchPayment(ctx, client, "", id, "", nil)
		if err != nil {
			return fmt.Errorf("stopped waiting for payment %s: %w; provisioning continues; resume with `notte payment wait %s`", id, err, id)
		}
		instructions := ""
		if result.Status == "awaiting_connection" && result.ConnectionURL != nil {
			instructions = "Connect wallet: " + *result.ConnectionURL
			if result.ConnectionPhrase != nil {
				instructions += "\nPhrase: " + *result.ConnectionPhrase
			}
		}
		if result.Status == "awaiting_approval" && result.ApprovalURL != nil {
			instructions = "Approve spending: " + *result.ApprovalURL
		}
		if result.NextAction != nil {
			instructions = "Wallet verification required: " + result.NextAction.Type
			if result.NextAction.ActionURL != nil {
				instructions += "\nComplete verification: " + *result.NextAction.ActionURL
			}
			if result.NextAction.Resolution == "auto_resume" {
				instructions += "\nWaiting for Link to verify completion; this request will resume automatically."
			} else {
				instructions += "\nAfter completing this action, run `notte payment request` again with a new idempotency key."
			}
		}
		if instructions != "" && instructions != lastInstructions {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), instructions)
			lastInstructions = instructions
		}
		switch result.Status {
		case "ready":
			return GetFormatter().Print(result)
		case "declined", "expired", "failed", "closed":
			return fmt.Errorf("payment %s ended with status %s (error_code: %s)", id, result.Status, paymentErrorCode(result))
		case "requires_action":
			if result.NextAction == nil || result.NextAction.Resolution != "auto_resume" {
				return fmt.Errorf("payment %s requires a new request after wallet verification; inspect with `notte payment status %s`", id, id)
			}
		case "creating", "awaiting_connection", "awaiting_approval", "approved", "provisioning":
		default:
			return fmt.Errorf("unknown payment status %q; inspect with `notte payment status %s`", result.Status, id)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("stopped waiting for payment %s: %w; provisioning continues; resume with `notte payment wait %s`", id, ctx.Err(), id)
		case <-timer.C:
		}
	}
}

func paymentErrorCode(p *api.PaymentStatus) string {
	if p.ErrorCode != nil {
		return *p.ErrorCode
	}
	return "none"
}
