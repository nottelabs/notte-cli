# Account Management Reference

Complete guide to managing personas and vaults for authentication and identity management.

## Overview

Notte provides two complementary systems for managing identities and credentials:

| Feature | Personas | Vaults |
|---------|----------|--------|
| Purpose | Auto-generated test identities | Store your own credentials |
| Email | Platform-generated inbox | Your email addresses |
| Phone | Platform-provided numbers | N/A |
| Credentials | Auto-managed | User-provided |
| Use case | Testing, signups | Login to existing accounts |

## Notte Personas

Personas are auto-generated identities with real email addresses and optional phone numbers. Perfect for:
- Testing signup flows
- Creating test accounts
- Receiving verification codes
- End-to-end testing

### Creating Personas

```bash
# Basic persona (email only)
notte personas create

# With phone number
notte personas create --create-phone-number

# With associated vault for credentials
notte personas create --create-vault

# Full-featured persona
notte personas create --create-phone-number --create-vault
```

### Managing Personas

```bash
# List all personas
notte personas list

# View persona details
notte personas show --id <persona-id>

# Delete persona
notte personas delete --id <persona-id>
```

### Receiving Emails

Personas have real email inboxes that receive messages:

```bash
# List emails received by persona
notte personas emails --id <persona-id>
```

Example response:
```json
{
  "emails": [
    {
      "id": "email_123",
      "from": "noreply@example.com",
      "subject": "Verify your email",
      "received_at": "2024-01-15T10:30:00Z",
      "body": "Your verification code is: 123456"
    }
  ]
}
```

### Receiving SMS

For personas with phone numbers:

```bash
# List SMS messages
notte personas sms --id <persona-id>
```

Example response:
```json
{
  "messages": [
    {
      "id": "sms_456",
      "from": "+1234567890",
      "body": "Your verification code is 789012",
      "received_at": "2024-01-15T10:31:00Z"
    }
  ]
}
```

### Phone Number Management

```bash
# Add phone number to existing persona
notte personas phone-create --id <persona-id>

# Remove phone number
notte personas phone-delete --id <persona-id>
```

### Persona Workflow Example

```bash
# Create persona for testing
PERSONA=$(notte personas create --create-phone-number -o json)
PERSONA_ID=$(echo "$PERSONA" | jq -r '.id')
EMAIL=$(echo "$PERSONA" | jq -r '.email')
PHONE=$(echo "$PERSONA" | jq -r '.phone_number')

# Start browser session
notte sessions start

# Fill signup form
notte page goto "https://example.com/signup"
notte page fill "@email" "$EMAIL"
notte page fill "@phone" "$PHONE"
notte page click "@submit"

# Wait for verification email
sleep 10

# Get verification code from email
CODE=$(notte personas emails --id "$PERSONA_ID" -o json | \
  jq -r '.emails[0].body' | \
  grep -oE '[0-9]{6}')

# Enter verification code
notte page fill "@verification-code" "$CODE"
notte page click "@verify"

# Cleanup
notte sessions stop
```

## User-Provided Vaults

Vaults store your own credentials for automated login to existing accounts.

### Creating Vaults

```bash
# Create vault
notte vaults create

# With custom name
notte vaults create --name "Work Accounts"
```

### Managing Vaults

```bash
# List vaults
notte vaults list

# Update vault name
notte vaults update --id <vault-id> --name "Personal Accounts"

# Delete vault
notte vaults delete --id <vault-id>
```

## Credential Management

### Adding Credentials

Store credentials for specific URLs:

```bash
# Basic credentials
notte vaults credentials add \
  --id <vault-id> \
  --url "https://example.com" \
  --password "mypassword"

# With email
notte vaults credentials add \
  --id <vault-id> \
  --url "https://example.com" \
  --email "user@example.com" \
  --password "mypassword"

# With username (for sites that use username instead of email)
notte vaults credentials add \
  --id <vault-id> \
  --url "https://example.com" \
  --username "myusername" \
  --password "mypassword"

# With MFA secret for TOTP
notte vaults credentials add \
  --id <vault-id> \
  --url "https://example.com" \
  --email "user@example.com" \
  --password "mypassword" \
  --mfa-secret "JBSWY3DPEHPK3PXP"
```

### Listing Credentials

```bash
notte vaults credentials list --id <vault-id>
```

Note: Passwords are not returned in list output for security.

### Getting Credentials for a URL

```bash
notte vaults credentials get --id <vault-id> --url "https://example.com"
```

Returns credentials matching the URL.

### Deleting Credentials

```bash
notte vaults credentials delete --id <vault-id> --url "https://example.com"
```

## MFA/TOTP Support

When you add an `--mfa-secret`, Notte can automatically generate TOTP codes:

```bash
# Add credentials with MFA secret
notte vaults credentials add \
  --id <vault-id> \
  --url "https://secure.example.com" \
  --email "user@example.com" \
  --password "password123" \
  --mfa-secret "JBSWY3DPEHPK3PXP"

# During automation, TOTP codes are generated automatically
# when the site requests 2FA
```

The MFA secret is the base32-encoded key shown when setting up authenticator apps (usually displayed as a QR code or "manual entry" key).

## Credit Card Storage

For e-commerce automation and checkout testing:

### Set Credit Card

```bash
notte vaults card-set \
  --id <vault-id> \
  --number "4111111111111111" \
  --expiry "12/25" \
  --cvv "123" \
  --name "John Doe"
```

### Get Credit Card

```bash
notte vaults card --id <vault-id>
```

### Delete Credit Card

```bash
notte vaults card-delete --id <vault-id>
```

## Authentication Patterns

### When to Use Personas

Use personas when you need:
- **New accounts**: Testing signup flows
- **Disposable identities**: One-time verifications
- **Email/SMS verification**: Need to receive codes
- **Testing**: Creating accounts for test scenarios

```bash
# Signup flow testing
notte personas create --create-phone-number
# → Use generated email/phone for signup
# → Check personas emails/sms for verification codes
```

### When to Use Vaults

Use vaults when you need:
- **Existing accounts**: Login to your accounts
- **Persistent credentials**: Same credentials across sessions
- **MFA automation**: Auto-generate TOTP codes
- **Payment flows**: Stored credit cards

```bash
# Login automation
notte vaults credentials add --id <vault-id> \
  --url "https://dashboard.example.com" \
  --email "myreal@email.com" \
  --password "myrealpassword" \
  --mfa-secret "MYREALMFASECRET"
# → Navigate to login page
# → Vault credentials auto-fill
# → TOTP generated automatically
```

### Combined Pattern

Use both for complex flows:

```bash
# Create persona for new account testing
notte personas create --create-vault --create-phone-number

# The persona's vault is linked and can store credentials
# created during the signup process

# After signup completes, credentials are saved to the
# persona's vault for subsequent logins
```

## Security Considerations

### Credential Storage

- Credentials are encrypted at rest
- API key controls access to your vaults
- MFA secrets enable automatic TOTP but require secure storage

### Best Practices

1. **Use separate vaults** for different purposes:
   ```bash
   notte vaults create --name "Production"
   notte vaults create --name "Staging"
   notte vaults create --name "Testing"
   ```

2. **Don't share API keys** - each user should have their own

3. **Use personas for testing** - don't test with real credentials

4. **Rotate credentials** - update vault credentials when you change passwords

5. **Clean up test personas** - delete when no longer needed:
   ```bash
   notte personas delete --id <persona-id>
   ```

## Complete Example: Authenticated Data Collection

```bash
#!/bin/bash
set -euo pipefail

# Setup: Create vault and add credentials (one-time)
# notte vaults create --name "Analytics Dashboard"
# notte vaults credentials add --id <vault-id> \
#   --url "https://analytics.example.com" \
#   --email "analyst@company.com" \
#   --password "securepassword" \
#   --mfa-secret "ANALYTICSTOTP"

VAULT_ID="vault_abc123"

# Start session
notte sessions start

# Navigate to login - vault credentials auto-fill
notte page goto "https://analytics.example.com/login"
notte page click "@submit-login"

# Wait for MFA (TOTP auto-generated from vault)
notte page wait 2000

# Now logged in, collect data
notte page goto "https://analytics.example.com/reports/weekly"
REPORT=$(notte sessions scrape --instructions "Extract the weekly metrics summary")

# Save cookies for faster future logins
notte sessions cookies -o json > analytics_cookies.json

# Cleanup
notte sessions stop

echo "Report collected: $REPORT"
```
