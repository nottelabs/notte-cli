package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/muesli/termenv"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/output"
)

// render prints a command's result. In JSON mode that is the value itself,
// unchanged. In text mode it is the view designed for the value's type, and
// for a type without one, the generic labelled output (output.TextFormatter).
// A new API response therefore prints readably before anyone designs a view
// for it, and designing one never changes what scripts and agents read.
func render(v any) error {
	text := textFormatter()
	if text == nil {
		return GetFormatter().Print(v)
	}
	now := time.Now()
	switch x := v.(type) {
	case []api.FunctionListItemResponse:
		return viewFunctions(text, x, now)
	case *api.FunctionResponse:
		if x != nil {
			return viewFunction(text, functionView{
				ID: x.FunctionId, Name: deref(x.Name), Status: x.Status, Description: deref(x.Description),
				Latest: x.LatestVersion, Versions: x.Versions, Runtime: fmt.Sprint(deref(x.DefaultRuntime)),
				Cron: deref(x.ScheduleCron), ScheduleState: deref(x.ScheduleState), Variables: deref(x.Variables),
				Secrets: deref(x.RequiredSecrets), Shared: deref(x.Shared), Published: deref(x.Published),
				SelfHealing: deref(x.SelfHealing), Protected: deref(x.DeletionProtected),
				Created: x.CreatedAt.Time, Updated: x.UpdatedAt.Time,
			}, now)
		}
	case *api.FunctionWithLinkResponse:
		if x != nil {
			return viewFunction(text, functionView{
				ID: x.FunctionId, Name: deref(x.Name), Status: x.Status, Description: deref(x.Description),
				Latest: x.LatestVersion, Versions: x.Versions, Runtime: fmt.Sprint(deref(x.DefaultRuntime)),
				Cron: deref(x.ScheduleCron), ScheduleState: deref(x.ScheduleState), Variables: deref(x.Variables),
				Secrets: deref(x.RequiredSecrets), Shared: deref(x.Shared), Published: deref(x.Published),
				SelfHealing: deref(x.SelfHealing), Protected: deref(x.DeletionProtected),
				Created: x.CreatedAt.Time, Updated: x.UpdatedAt.Time, SourceURL: x.Url,
			}, now)
		}
	case []api.FunctionRunListItemResponse:
		return viewFunctionRuns(text, x, now)
	case *api.GetFunctionRunResponse:
		if x != nil {
			return viewFunctionRun(text, x, now)
		}
	case *api.FunctionRuntimeHealthResponse:
		if x != nil {
			return viewRuntimeHealth(text, x)
		}
	case []api.SecretMetadata:
		return viewSecrets(text, x, now)
	case *api.FunctionScheduleListResponse:
		if x != nil {
			return viewSchedules(text, x, now)
		}
	case []api.PersonaResponse:
		return viewPersonas(text, x, now)
	case *api.PersonaResponse:
		if x != nil {
			return viewPersona(text, x, now)
		}
	case *[]api.EmailResponse:
		return viewEmails(text, deref(x), now)
	case *[]api.SMSResponse:
		return viewSMS(text, deref(x), now)
	case []api.ProfileResponse:
		return viewProfiles(text, x, now)
	case *api.ProfileResponse:
		if x != nil {
			return viewProfile(text, x, now)
		}
	case []api.Vault:
		return viewVaults(text, x, now)
	case *api.Vault:
		if x != nil {
			return viewVault(text, x, now)
		}
	case []api.Credential:
		return viewCredentials(text, x)
	case *api.GetCookiesResponse:
		if x != nil {
			return viewCookies(text, x.Cookies, now)
		}
	case []sessionFile:
		return viewFiles(text, x, now)
	case *api.UsageResponse:
		if x != nil {
			return viewUsage(text, x)
		}
	case []api.UsageLog:
		return viewUsageLogs(text, x, now)
	case []tunnelRow:
		return viewTunnels(text, x)
	case *api.PaymentStatus:
		if x != nil {
			return viewPayment(text, x, now)
		}
	case *api.WalletConnectionStatus:
		if x != nil {
			return viewWallet(text, x, now)
		}
	case *api.UpdateFunctionRunResponse:
		if x != nil {
			return viewRunUpdate(text, x, now)
		}
	case *api.FunctionSchedule:
		if x != nil {
			return viewSchedule(text, x, now)
		}
	case *api.SecretMetadata:
		if x != nil {
			return viewSecret(text, x, now)
		}
	case *api.SecretValueResponse:
		if x != nil {
			// The value alone, so that it can be piped or copied.
			_, err := fmt.Fprintln(text.Writer, x.Value)
			return err
		}
	case *api.AddCredentialsResponse:
		printHint(text, "notte vaults credentials list --vault-id "+vaultID)
		return nil
	case *api.GetCredentialsResponse:
		if x != nil {
			return viewCredential(text, x)
		}
	case *api.ProfileCookiesImportResponse:
		if x != nil {
			if _, err := fmt.Fprintf(text.Writer, "  %s   %d (%s)\n", text.Label("Cookies"), x.CookiesCount, x.Mode); err != nil {
				return err
			}
		}
		printHint(text, "notte profiles cookies --profile-id "+profileID)
		return nil
	case *api.ExecutionResponse:
		printHint(text, "notte sessions cookies --session-id "+sessionID)
		return nil
	case *api.SessionOffsetResponse:
		if x != nil {
			if err := printDetails(text, "Session "+sessionID, [][2]string{
				{"Steps", strconv.Itoa(x.Offset)},
				{"Console", consoleLink("sessions", sessionID)},
			}); err != nil {
				return err
			}
			printHint(text, "notte sessions code --session-id "+sessionID)
			return nil
		}
	case *api.HealthResponse:
		if x != nil {
			if err := printDetails(text, "Notte API", [][2]string{
				{"Status", statusText(text, deref(x.Status))},
				{"Version", deref(x.Version)},
				{"Description", deref(x.Description)},
				{"Console", output.Link(strings.TrimSuffix(auth.ConsoleURL(), "/"), consoleLabel)},
			}); err != nil {
				return err
			}
			printHint(text, "notte auth status")
			return nil
		}
	}
	return text.Print(v)
}

// versionLabel prints a numbered version the way people say it, "v3", and
// any other version name (a timestamp, a tag) as it is.
func versionLabel(version string) string {
	if _, err := strconv.Atoi(version); err == nil && len(version) <= 4 {
		return "v" + version
	}
	return version
}

func viewFunctions(text *output.TextFormatter, items []api.FunctionListItemResponse, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{
			item.FunctionId, nameCell(deref(item.Name)), statusText(text, item.Status),
			versionLabel(item.LatestVersion), relativeTime(item.UpdatedAt.Time, now),
			consoleLink("functions", item.FunctionId),
		})
	}
	if err := listTable(text, GetCurrentFunctionID(), []string{countHeader("FUNCTION", len(items), false), "NAME", "STATUS", "VERSION", "UPDATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte functions run --function-id <id>")
	return nil
}

// nameCell is a name in a table: shortened to fit, "-" when there is none.
func nameCell(name string) string {
	if name == "" {
		return "-"
	}
	return shorten(name, 32)
}

// functionView is what the two function responses have in common.
type functionView struct {
	ID, Name, Status, Description, Latest, Runtime, Cron, ScheduleState, SourceURL string
	Versions, Secrets                                                              []string
	Variables                                                                      []api.ParameterInfo
	Shared, Published, SelfHealing, Protected                                      bool
	Created, Updated                                                               time.Time
}

func viewFunction(text *output.TextFormatter, f functionView, now time.Time) error {
	title := f.ID
	if f.Name != "" {
		title = f.Name
	}
	version := versionLabel(f.Latest)
	if len(f.Versions) > 1 {
		version += fmt.Sprintf(" (of %d)", len(f.Versions))
	}
	variables := make([]string, 0, len(f.Variables))
	for _, variable := range f.Variables {
		line := variable.Name
		if variable.Type != nil && *variable.Type != "" {
			line += " (" + *variable.Type + ")"
		}
		if variable.Default != nil {
			line += " = " + *variable.Default
		}
		variables = append(variables, line)
	}
	schedule := f.Cron
	if schedule != "" && f.ScheduleState != "" {
		schedule += "  " + statusText(text, f.ScheduleState)
	}
	var flags []string
	for flag, on := range map[string]bool{"shared": f.Shared, "published": f.Published, "self-healing": f.SelfHealing, "protected from deletion": f.Protected} {
		if on {
			flags = append(flags, flag)
		}
	}
	sort.Strings(flags)
	source := ""
	if f.SourceURL != "" {
		source = output.Link(f.SourceURL, "download source ↗")
	}
	if err := printDetails(text, title, [][2]string{
		{"Function", f.ID},
		{"Status", statusText(text, f.Status)},
		{"Version", version},
		{"Runtime", f.Runtime},
		{"Description", f.Description},
		{"Variables", strings.Join(variables, "\n")},
		{"Secrets", strings.Join(f.Secrets, ", ")},
		{"Schedule", schedule},
		{"Flags", strings.Join(flags, ", ")},
		{"Created", relativeTime(f.Created, now)},
		{"Updated", relativeTime(f.Updated, now)},
		{"Source", source},
		{"Console", consoleLink("functions", f.ID)},
	}); err != nil {
		return err
	}
	printHint(text, "notte functions run --function-id "+f.ID)
	return nil
}

func viewFunctionRuns(text *output.TextFormatter, items []api.FunctionRunListItemResponse, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		took := "-"
		if item.Status != api.FunctionRunListItemResponseStatusActive && item.UpdatedAt.After(item.CreatedAt.Time) {
			took = formatDuration(item.UpdatedAt.Sub(item.CreatedAt.Time))
		}
		session := deref(item.SessionId)
		if session == "" {
			session = "-"
		}
		rows = append(rows, []string{item.FunctionRunId, runStatus(text, string(item.Status)), took, session, relativeTime(item.CreatedAt.Time, now), consoleLink("functions", item.FunctionId, "runs", item.FunctionRunId)})
	}
	if err := listTable(text, "", []string{countHeader("RUN", len(items), false), "STATUS", "TOOK", "SESSION", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	if len(items) > 0 {
		printHint(text, fmt.Sprintf("notte functions run-metadata --function-id %s --run-id <id>", items[0].FunctionId))
	}
	return nil
}

// runStatus names a run's state: the API calls a run that finished "closed".
func runStatus(text *output.TextFormatter, status string) string {
	if status == "closed" {
		return text.Colorize("finished", termenv.ANSIGreen)
	}
	return statusText(text, status)
}

func viewFunctionRun(text *output.TextFormatter, run *api.GetFunctionRunResponse, now time.Time) error {
	took := ""
	if run.Status != api.GetFunctionRunResponseStatusActive && run.UpdatedAt.After(run.CreatedAt.Time) {
		took = formatDuration(run.UpdatedAt.Sub(run.CreatedAt.Time))
	}
	variables := ""
	if run.Variables != nil && len(*run.Variables) > 0 {
		if data, err := json.Marshal(*run.Variables); err == nil {
			variables = string(data)
		}
	}
	if err := printDetails(text, "Run "+run.FunctionRunId, [][2]string{
		{"Function", run.FunctionId},
		{"Status", runStatus(text, string(run.Status))},
		{"Version", versionLabel(deref(run.FunctionVersion))},
		{"Session", deref(run.SessionId)},
		{"Variables", variables},
		{"Took", took},
		{"Created", relativeTime(run.CreatedAt.Time, now)},
		{"Console", consoleLink("functions", run.FunctionId, "runs", run.FunctionRunId)},
	}); err != nil {
		return err
	}
	// The result is what the run was for: print it whole, as JSON when it is.
	if result := strings.TrimSpace(deref(run.Result)); result != "" {
		var parsed any
		if json.Unmarshal([]byte(result), &parsed) == nil {
			if pretty, err := json.MarshalIndent(parsed, "", "  "); err == nil {
				result = string(pretty)
			}
		}
		if _, err := fmt.Fprintf(text.Writer, "\n%s\n%s\n\n", text.Label("Result"), result); err != nil {
			return err
		}
	}
	printHint(text, "notte functions runs --function-id "+run.FunctionId)
	return nil
}

// viewRunUpdate is a run after a change to it (stopped, metadata updated).
func viewRunUpdate(text *output.TextFormatter, run *api.UpdateFunctionRunResponse, now time.Time) error {
	status := ""
	if run.Status != nil {
		status = runStatus(text, string(*run.Status))
	}
	if err := printDetails(text, "Run "+run.FunctionRunId, [][2]string{
		{"Function", run.FunctionId},
		{"Status", status},
		{"Updated", relativeTime(run.UpdatedAt.Time, now)},
		{"Console", consoleLink("functions", run.FunctionId, "runs", run.FunctionRunId)},
	}); err != nil {
		return err
	}
	printHint(text, fmt.Sprintf("notte functions run-metadata --function-id %s --run-id %s", run.FunctionId, run.FunctionRunId))
	return nil
}

func viewSchedule(text *output.TextFormatter, schedule *api.FunctionSchedule, now time.Time) error {
	variables := ""
	if schedule.Variables != nil && len(*schedule.Variables) > 0 {
		if data, err := json.Marshal(*schedule.Variables); err == nil {
			variables = string(data)
		}
	}
	created := ""
	if t, err := time.Parse(time.RFC3339, deref(schedule.CreatedAt)); err == nil {
		created = relativeTime(t, now)
	}
	if err := printDetails(text, "Schedule "+schedule.ScheduleId, [][2]string{
		{"Function", functionID},
		{"Cron", schedule.Cron},
		{"Variables", variables},
		{"Created", created},
		{"Console", consoleLink("functions", functionID, "schedule")},
	}); err != nil {
		return err
	}
	printHint(text, "notte functions schedules list --function-id "+functionID)
	return nil
}

func viewSecret(text *output.TextFormatter, secret *api.SecretMetadata, now time.Time) error {
	used := "never"
	if t, err := time.Parse(time.RFC3339, deref(secret.LastUsedAt)); err == nil {
		used = relativeTime(t, now)
	}
	if err := printDetails(text, secret.Name, [][2]string{
		{"Secret", secret.Id},
		{"Value", secret.KeyHint},
		{"Last used", used},
		{"Created", relativeTime(secret.CreatedAt.Time, now)},
		{"Console", consoleLink("settings", "function-secrets")},
	}); err != nil {
		return err
	}
	printHint(text, "notte functions secrets list")
	return nil
}

// viewCredential prints one site's saved login, password included: showing
// it is what the command is for.
func viewCredential(text *output.TextFormatter, got *api.GetCredentialsResponse) error {
	c := got.Credentials
	if err := printDetails(text, vaultCredentialsGetURL, [][2]string{
		{"Vault", vaultID},
		{"Username", deref(c.Username)},
		{"Email", deref(c.Email)},
		{"Password", c.Password},
		{"MFA secret", deref(c.MfaSecret)},
		{"Console", consoleLink("vaults")},
	}); err != nil {
		return err
	}
	printHint(text, "notte vaults credentials list --vault-id "+vaultID)
	return nil
}

// expiry prints an RFC 3339 time relative to now, or nothing.
func expiry(stamp *string, now time.Time) string {
	if t, err := time.Parse(time.RFC3339, deref(stamp)); err == nil {
		return relativeTime(t, now)
	}
	return deref(stamp)
}

func viewPayment(text *output.TextFormatter, p *api.PaymentStatus, now time.Time) error {
	amount := p.Amount.String()
	if amount != "" && p.Currency != "" {
		amount += " " + strings.ToUpper(p.Currency)
	}
	merchant := p.MerchantName
	if p.MerchantURL != "" {
		merchant = strings.TrimSpace(merchant + "  " + p.MerchantURL)
	}
	action := ""
	if p.NextAction != nil {
		action = p.NextAction.Type
		if url := deref(p.NextAction.ActionURL); url != "" {
			action += "  " + url
		}
	}
	console := ""
	if p.SessionID != "" {
		console = consoleLink("sessions", p.SessionID)
	}
	if err := printDetails(text, "Payment "+p.ID, [][2]string{
		{"Status", statusText(text, p.Status)},
		{"Purchase", statusText(text, p.PurchaseStatus)},
		{"Mode", p.Mode},
		{"Amount", amount},
		{"Merchant", merchant},
		{"Session", p.SessionID},
		{"Vault", deref(p.VaultID)},
		{"Connect", deref(p.ConnectionURL)},
		{"Phrase", deref(p.ConnectionPhrase)},
		{"Approve", deref(p.ApprovalURL)},
		{"Verify", action},
		{"Error", deref(p.ErrorCode)},
		{"Expires", expiry(p.ExpiresAt, now)},
		{"Console", console},
	}); err != nil {
		return err
	}
	next := "notte payment wait " + p.ID
	if p.Status == "ready" {
		next = "notte page observe --session-id " + p.SessionID
	}
	printHint(text, next)
	return nil
}

func viewWallet(text *output.TextFormatter, w *api.WalletConnectionStatus, now time.Time) error {
	if err := printDetails(text, "Link wallet", [][2]string{
		{"Connection", w.ID},
		{"Status", statusText(text, w.Status)},
		{"Mode", w.Mode},
		{"Connect", deref(w.ConnectionURL)},
		{"Phrase", deref(w.ConnectionPhrase)},
		{"Error", deref(w.ErrorCode)},
		{"Expires", expiry(w.ExpiresAt, now)},
	}); err != nil {
		return err
	}
	next := paymentConnectCommand(w.Mode)
	if w.Status == "connected" {
		next = "notte payment request --help"
	}
	printHint(text, next)
	return nil
}

func viewRuntimeHealth(text *output.TextFormatter, health *api.FunctionRuntimeHealthResponse) error {
	latency := ""
	if health.LatencyMs != nil {
		latency = formatDuration(time.Duration(float64(*health.LatencyMs) * float64(time.Millisecond)))
	}
	var packages []string
	for _, pkg := range deref(health.Packages) {
		name := deref(pkg.Package)
		if name == "" {
			name = pkg.ImportName
		}
		if pkg.Installed != nil && !*pkg.Installed {
			continue
		}
		if version := deref(pkg.Version); version != "" {
			name += " " + version
		}
		packages = append(packages, name)
	}
	if err := printDetails(text, "Function runtime", [][2]string{
		{"Status", statusText(text, string(health.Status))},
		{"Reachable", yesNo(health.Reachable)},
		{"Latency", latency},
		{"Python", deref(health.PythonVersion)},
		{"Error", deref(health.Error)},
		{"Packages", strings.Join(packages, "\n")},
	}); err != nil {
		return err
	}
	printHint(text, "notte functions health -o json   (adds standard library modules and reserved names)")
	return nil
}

func viewSecrets(text *output.TextFormatter, items []api.SecretMetadata, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		used := "never"
		if t, err := time.Parse(time.RFC3339, deref(item.LastUsedAt)); err == nil {
			used = relativeTime(t, now)
		}
		rows = append(rows, []string{item.Id, item.Name, item.KeyHint, used, relativeTime(item.CreatedAt.Time, now), consoleLink("settings", "function-secrets")})
	}
	if err := listTable(text, "", []string{countHeader("SECRET", len(items), false), "NAME", "VALUE", "LAST USED", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte functions secrets set <name> <value>")
	return nil
}

func viewSchedules(text *output.TextFormatter, list *api.FunctionScheduleListResponse, now time.Time) error {
	if len(list.Schedules) == 0 {
		_, err := fmt.Fprintln(text.Writer, withEmptyFace("No schedules"))
		return err
	}
	rows := make([][]string, 0, len(list.Schedules))
	for _, schedule := range list.Schedules {
		variables := "-"
		if schedule.Variables != nil && len(*schedule.Variables) > 0 {
			if data, err := json.Marshal(*schedule.Variables); err == nil {
				variables = shorten(string(data), 60)
			}
		}
		created := "-"
		if t, err := time.Parse(time.RFC3339, deref(schedule.CreatedAt)); err == nil {
			created = relativeTime(t, now)
		}
		rows = append(rows, []string{schedule.ScheduleId, schedule.Cron, variables, created, consoleLink("functions", functionID, "schedule")})
	}
	if err := listTable(text, "", []string{countHeader("SCHEDULE", len(rows), false), "CRON", "VARIABLES", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	if state := deref(list.State); state != "" {
		line := "Schedules are " + statusText(text, state)
		if reason := deref(list.PausedReason); reason != "" {
			line += ": " + reason
		}
		if _, err := fmt.Fprintln(text.Writer, line); err != nil {
			return err
		}
	}
	printHint(text, "notte functions schedules add --function-id "+functionID+" --cron <expression>")
	return nil
}

func personaName(p *api.PersonaResponse) string {
	if name := deref(p.Name); name != "" {
		return name
	}
	return strings.TrimSpace(p.FirstName + " " + p.LastName)
}

func viewPersonas(text *output.TextFormatter, items []api.PersonaResponse, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		phone := deref(item.PhoneNumber)
		if phone == "" {
			phone = "-"
		}
		rows = append(rows, []string{item.PersonaId, nameCell(personaName(&item)), statusText(text, item.Status), item.Email, phone, relativeTime(item.CreatedAt.Time, now), consoleLink("personas")})
	}
	if err := listTable(text, "", []string{countHeader("PERSONA", len(items), false), "NAME", "STATUS", "EMAIL", "PHONE", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte personas emails --persona-id <id>")
	return nil
}

func viewPersona(text *output.TextFormatter, p *api.PersonaResponse, now time.Time) error {
	inbox := ""
	if p.InternalEmail != "" && p.InternalEmail != p.Email {
		inbox = p.InternalEmail
	}
	if err := printDetails(text, personaName(p), [][2]string{
		{"Persona", p.PersonaId},
		{"Status", statusText(text, p.Status)},
		{"Name", strings.TrimSpace(p.FirstName + " " + p.LastName)},
		{"Email", p.Email},
		{"Inbox", inbox},
		{"Phone", deref(p.PhoneNumber)},
		{"Vault", deref(p.VaultId)},
		{"Created", relativeTime(p.CreatedAt.Time, now)},
		{"Console", consoleLink("personas")},
	}); err != nil {
		return err
	}
	printHint(text, "notte personas emails --persona-id "+p.PersonaId)
	return nil
}

// viewEmails prints each message whole: reading a code or a link out of an
// email is the reason to list them.
func viewEmails(text *output.TextFormatter, emails []api.EmailResponse, now time.Time) error {
	var b strings.Builder
	for i, email := range emails {
		if i > 0 {
			b.WriteString("\n")
		}
		from := deref(email.SenderEmail)
		if name := deref(email.SenderName); name != "" && from != "" {
			from = name + " <" + from + ">"
		} else if name != "" {
			from = name
		}
		var attachments []string
		for _, attachment := range deref(email.Attachments) {
			attachments = append(attachments, fmt.Sprintf("%s (%s)", output.Link(attachment.DownloadUrl, attachment.Filename), formatBytes(int64(attachment.SizeBytes))))
		}
		var details strings.Builder
		_ = printDetails(&output.TextFormatter{Writer: &details, NoColor: text.NoColor}, email.Subject, [][2]string{
			{"Email", email.EmailId},
			{"From", from},
			{"Created", relativeTime(email.CreatedAt.Time, now)},
			{"Attachments", strings.Join(attachments, "\n")},
		})
		b.WriteString(details.String())
		body := strings.TrimSpace(deref(email.TextContent))
		if body == "" {
			body = strings.TrimSpace(deref(email.HtmlContent))
		}
		if body != "" {
			b.WriteString("\n  " + strings.ReplaceAll(body, "\n", "\n  ") + "\n")
		}
	}
	_, err := io.WriteString(text.Writer, b.String())
	return err
}

func viewSMS(text *output.TextFormatter, messages []api.SMSResponse, now time.Time) error {
	rows := make([][]string, 0, len(messages))
	for _, message := range messages {
		sender := deref(message.Sender)
		if sender == "" {
			sender = "-"
		}
		// The body is the last column, so a long message is still whole.
		rows = append(rows, []string{message.SmsId, sender, relativeTime(message.CreatedAt.Time, now), strings.Join(strings.Fields(message.Body), " ")})
	}
	// No console column: the message is last so that it is never cut.
	return listTable(text, "", []string{countHeader("SMS", len(messages), false), "FROM", "CREATED", "MESSAGE"}, rows)
}

func viewProfiles(text *output.TextFormatter, items []api.ProfileResponse, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, []string{item.ProfileId, nameCell(deref(item.Name)), domainSummary(deref(item.PersistedDomains), 3), relativeTime(item.UpdatedAt.Time, now), consoleLink("profiles")})
	}
	if err := listTable(text, "", []string{countHeader("PROFILE", len(items), false), "NAME", "SITES", "UPDATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte sessions start --profile-id <id>")
	return nil
}

// domainSummary lists the first few domains and counts the rest.
func domainSummary(domains []string, max int) string {
	if len(domains) == 0 {
		return "-"
	}
	if len(domains) <= max {
		return strings.Join(domains, ", ")
	}
	return fmt.Sprintf("%s +%d more", strings.Join(domains[:max], ", "), len(domains)-max)
}

func viewProfile(text *output.TextFormatter, p *api.ProfileResponse, now time.Time) error {
	title := p.ProfileId
	if name := deref(p.Name); name != "" {
		title = name
	}
	if err := printDetails(text, title, [][2]string{
		{"Profile", p.ProfileId},
		{"Sites", strings.Join(deref(p.PersistedDomains), "\n")},
		{"Created", relativeTime(p.CreatedAt.Time, now)},
		{"Updated", relativeTime(p.UpdatedAt.Time, now)},
		{"Console", consoleLink("profiles")},
	}); err != nil {
		return err
	}
	printHint(text, "notte sessions start --profile-id "+p.ProfileId)
	return nil
}

func viewVaults(text *output.TextFormatter, items []api.Vault, now time.Time) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		kind := "shared"
		if deref(item.ForPersona) {
			kind = "persona"
		}
		rows = append(rows, []string{item.VaultId, nameCell(item.Name), kind, relativeTime(item.CreatedAt.Time, now), consoleLink("vaults")})
	}
	if err := listTable(text, "", []string{countHeader("VAULT", len(items), false), "NAME", "KIND", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte vaults credentials list --vault-id <id>")
	return nil
}

func viewVault(text *output.TextFormatter, v *api.Vault, now time.Time) error {
	kind := "shared"
	if deref(v.ForPersona) {
		kind = "belongs to a persona"
	}
	if err := printDetails(text, v.Name, [][2]string{
		{"Vault", v.VaultId},
		{"Kind", kind},
		{"Created", relativeTime(v.CreatedAt.Time, now)},
		{"Console", consoleLink("vaults")},
	}); err != nil {
		return err
	}
	printHint(text, "notte vaults credentials list --vault-id "+v.VaultId)
	return nil
}

func viewCredentials(text *output.TextFormatter, items []api.Credential) error {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		username, email := deref(item.Username), deref(item.Email)
		if username == "" {
			username = "-"
		}
		if email == "" {
			email = "-"
		}
		rows = append(rows, []string{item.Url, username, email, consoleLink("vaults")})
	}
	if err := listTable(text, "", []string{countHeader("SITE", len(items), false), "USERNAME", "EMAIL", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte vaults credentials add --vault-id "+vaultID+" --url <url> --password <password>")
	return nil
}

func viewCookies(text *output.TextFormatter, cookies []api.Cookie, now time.Time) error {
	if len(cookies) == 0 {
		_, err := fmt.Fprintln(text.Writer, withEmptyFace("No cookies"))
		return err
	}
	rows := make([][]string, 0, len(cookies))
	for _, cookie := range cookies {
		expires := "session"
		for _, at := range []*float32{cookie.Expires, cookie.ExpirationDate} {
			if at != nil && *at > 0 {
				expires = relativeTime(time.Unix(int64(*at), 0), now)
				break
			}
		}
		var flags []string
		if deref(cookie.Secure) {
			flags = append(flags, "secure")
		}
		if cookie.HttpOnly {
			flags = append(flags, "http-only")
		}
		if sameSite := deref(cookie.SameSite); sameSite != "" {
			flags = append(flags, "same-site "+strings.ToLower(sameSite))
		}
		rows = append(rows, []string{cookie.Name, cookie.Domain, cookie.Path, expires, strings.Join(flags, ", "), shorten(cookie.Value, 40)})
	}
	if err := listTable(text, "", []string{countHeader("COOKIE", len(cookies), false), "DOMAIN", "PATH", "EXPIRES", "FLAGS", "VALUE"}, rows); err != nil {
		return err
	}
	printHint(text, "add -o json for the full values")
	return nil
}

func viewFiles(text *output.TextFormatter, files []sessionFile, now time.Time) error {
	rows := make([][]string, 0, len(files))
	when := func(stamp string) string {
		if t, err := time.Parse(time.RFC3339, stamp); err == nil {
			return relativeTime(t, now)
		}
		return "-"
	}
	for _, file := range files {
		console := "-"
		if file.SessionID != "" {
			console = consoleLink("sessions", file.SessionID)
		}
		rows = append(rows, []string{file.ID, nameCell(file.Filename), formatBytes(file.Size), file.MimeType, when(file.ExpiresAt), when(file.CreatedAt), console})
	}
	if err := listTable(text, "", []string{countHeader("FILE", len(files), false), "NAME", "SIZE", "TYPE", "EXPIRES", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte files download <id>")
	return nil
}

func viewUsage(text *output.TextFormatter, usage *api.UsageResponse) error {
	plan := ""
	if raw, err := usage.PlanType.MarshalJSON(); err == nil {
		_ = json.Unmarshal(raw, &plan)
	}
	title := "Usage for " + usage.Period
	if plan != "" {
		title += "  " + text.Faint(plan+" plan")
	}
	money := func(v float32) string { return formatMoney(float64(v)) }
	limit := ""
	if usage.IsUsageLimitExceeded {
		limit = text.Colorize("reached", termenv.ANSIRed)
	}
	plural := func(n int, noun string) string {
		if n == 1 {
			return "1 " + noun
		}
		return strconv.Itoa(n) + " " + noun + "s"
	}
	if err := printDetails(text, title, [][2]string{
		{"Balance", money(usage.BalanceAmount)},
		{"  subscription", fmt.Sprintf("%s of %s", money(usage.SubscriptionBalanceAmount), money(usage.SubscriptionLimitAmount))},
		{"  top-up", money(usage.TopupBalanceAmount)},
		{"Spent", money(usage.TotalCost)},
		{"  browsers", fmt.Sprintf("%s  %s", money(usage.BrowserUsageCost), text.Faint(plural(usage.SessionCount, "session")))},
		{"  functions", fmt.Sprintf("%s  %s", money(usage.FunctionUsageCost), text.Faint(plural(usage.FunctionCount, "run")))},
		{"  LLM", money(usage.LlmUsageCost)},
		{"  proxies", money(usage.ProxyUsageCost)},
		{"Limit", limit},
		{"Console", consoleLink("billing")},
	}); err != nil {
		return err
	}
	printHint(text, "notte usage logs")
	return nil
}

func viewUsageLogs(text *output.TextFormatter, logs []api.UsageLog, now time.Time) error {
	rows := make([][]string, 0, len(logs))
	for _, log := range logs {
		rows = append(rows, []string{log.Endpoint, formatDuration(time.Duration(log.DurationMs) * time.Millisecond), relativeTime(log.CreatedAt.Time, now), consoleLink("logs")})
	}
	if err := listTable(text, "", []string{countHeader("REQUEST", len(logs), false), "TOOK", "CREATED", "CONSOLE"}, rows); err != nil {
		return err
	}
	printHint(text, "notte usage")
	return nil
}

func viewTunnels(text *output.TextFormatter, tunnels []tunnelRow) error {
	rows := make([][]string, 0, len(tunnels))
	for _, tunnel := range tunnels {
		rows = append(rows, []string{tunnel.Name, tunnel.ExitNode, tunnel.Tailnet, tunnel.Credentials, tunnel.Approved})
	}
	// Tunnels are saved on this machine; the console does not know them.
	if err := listTable(text, "", []string{countHeader("TUNNEL", len(tunnels), false), "EXIT NODE", "TAILNET", "CREDENTIALS", "APPROVED"}, rows); err != nil {
		return err
	}
	printHint(text, "notte sessions start --tunnel <name>")
	return nil
}
