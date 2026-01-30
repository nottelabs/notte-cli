package auth

const setupTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Notte CLI Setup</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Playfair+Display:wght@500;600&family=Inter:wght@300;400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #fafafa;
            --bg-card: #ffffff;
            --bg-input: #f4f4f5;
            --border: #e4e4e7;
            --border-focus: #00E06B;
            --text: #18181b;
            --text-muted: #71717a;
            --text-dim: #a1a1aa;
            --accent: #00E06B;
            --accent-dark: #00c45e;
            --accent-glow: rgba(0, 224, 107, 0.12);
            --success: #00E06B;
            --success-glow: rgba(0, 224, 107, 0.15);
            --error: #ef4444;
            --error-glow: rgba(239, 68, 68, 0.1);
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Inter', -apple-system, sans-serif;
            background: var(--bg);
            color: var(--text);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
            position: relative;
            overflow-x: hidden;
        }

        /* Concentric circles background */
        .bg-pattern {
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            width: 1200px;
            height: 1200px;
            pointer-events: none;
            z-index: 0;
            opacity: 0.4;
        }

        .bg-pattern circle {
            fill: none;
            stroke: #d4d4d8;
            stroke-width: 1;
        }

        /* Gradient accent */
        .bg-gradient {
            position: fixed;
            top: -30%;
            right: -20%;
            width: 800px;
            height: 800px;
            background: radial-gradient(circle, var(--accent-glow) 0%, transparent 70%);
            pointer-events: none;
            z-index: 0;
        }

        .container {
            width: 100%;
            max-width: 480px;
            position: relative;
            z-index: 1;
        }

        /* Terminal prompt header */
        .terminal-header {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 2rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8125rem;
            color: var(--text-muted);
        }

        .terminal-header::before {
            content: '$';
            color: var(--accent);
            font-weight: 500;
        }

        /* Logo section */
        .logo-section {
            text-align: center;
            margin-bottom: 2.5rem;
        }

        .logo {
            height: 32px;
            margin-bottom: 1.5rem;
            color: var(--text);
        }

        h1 {
            font-family: 'Playfair Display', Georgia, serif;
            font-size: 1.875rem;
            font-weight: 500;
            letter-spacing: -0.02em;
            margin-bottom: 0.5rem;
        }

        .subtitle {
            color: var(--text-muted);
            font-size: 0.9375rem;
            font-weight: 300;
        }

        /* Card */
        .card {
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 16px;
            padding: 2rem;
            box-shadow:
                0 1px 3px rgba(0, 0, 0, 0.04),
                0 6px 16px rgba(0, 0, 0, 0.04);
        }

        /* Form */
        .form-group {
            margin-bottom: 1.5rem;
        }

        label {
            display: block;
            font-size: 0.75rem;
            font-weight: 500;
            color: var(--text-muted);
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.06em;
        }

        input {
            width: 100%;
            padding: 0.875rem 1rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.875rem;
            background: var(--bg-input);
            border: 1.5px solid transparent;
            border-radius: 10px;
            color: var(--text);
            transition: all 0.2s ease;
        }

        input::placeholder {
            color: var(--text-dim);
        }

        input:hover {
            border-color: var(--border);
        }

        input:focus {
            outline: none;
            border-color: var(--border-focus);
            background: var(--bg-card);
            box-shadow: 0 0 0 3px var(--accent-glow);
        }

        .input-hint {
            font-size: 0.75rem;
            color: var(--text-dim);
            margin-top: 0.5rem;
        }

        .input-hint a {
            color: var(--accent-dark);
            text-decoration: none;
            font-weight: 500;
        }

        .input-hint a:hover {
            text-decoration: underline;
        }

        /* Button */
        .btn-group {
            display: flex;
            gap: 0.75rem;
            margin-top: 2rem;
        }

        button {
            flex: 1;
            padding: 0.875rem 1.5rem;
            font-family: 'Inter', sans-serif;
            font-size: 0.875rem;
            font-weight: 500;
            border-radius: 10px;
            cursor: pointer;
            transition: all 0.2s ease;
            border: none;
        }

        .btn-secondary {
            background: transparent;
            border: 1.5px solid var(--border);
            color: var(--text-muted);
        }

        .btn-secondary:hover {
            background: var(--bg-input);
            border-color: var(--text-dim);
            color: var(--text);
        }

        .btn-primary {
            background: var(--accent);
            color: #000;
            font-weight: 600;
        }

        .btn-primary:hover {
            background: var(--accent-dark);
            transform: translateY(-1px);
            box-shadow: 0 4px 12px var(--accent-glow);
        }

        .btn-primary:active {
            transform: translateY(0);
        }

        button:disabled {
            opacity: 0.5;
            cursor: not-allowed;
            transform: none !important;
        }

        /* Status messages */
        .status {
            margin-top: 1.5rem;
            padding: 0.875rem 1rem;
            border-radius: 10px;
            font-size: 0.8125rem;
            display: none;
            align-items: center;
            gap: 0.625rem;
            font-family: 'JetBrains Mono', monospace;
        }

        .status.show {
            display: flex;
        }

        .status.loading {
            background: var(--accent-glow);
            border: 1px solid rgba(0, 224, 107, 0.2);
            color: var(--accent-dark);
        }

        .status.success {
            background: var(--success-glow);
            border: 1px solid rgba(0, 224, 107, 0.25);
            color: var(--accent-dark);
        }

        .status.error {
            background: var(--error-glow);
            border: 1px solid rgba(239, 68, 68, 0.2);
            color: var(--error);
        }

        .spinner {
            width: 14px;
            height: 14px;
            border: 2px solid currentColor;
            border-top-color: transparent;
            border-radius: 50%;
            animation: spin 0.7s linear infinite;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        /* Help section */
        .help-section {
            margin-top: 2rem;
            padding-top: 1.5rem;
            border-top: 1px solid var(--border);
        }

        .help-title {
            font-size: 0.6875rem;
            font-weight: 500;
            color: var(--text-dim);
            text-transform: uppercase;
            letter-spacing: 0.08em;
            margin-bottom: 1rem;
        }

        .help-steps {
            display: flex;
            flex-direction: column;
            gap: 0.625rem;
        }

        .help-step {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            font-size: 0.8125rem;
            color: var(--text-muted);
        }

        .help-num {
            width: 20px;
            height: 20px;
            background: var(--bg-input);
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.6875rem;
            font-weight: 500;
            color: var(--text-dim);
            flex-shrink: 0;
        }

        /* Footer */
        .footer {
            text-align: center;
            margin-top: 2rem;
            font-size: 0.8125rem;
            color: var(--text-dim);
        }

        .footer a {
            color: var(--text-muted);
            text-decoration: none;
            transition: color 0.2s;
        }

        .footer a:hover {
            color: #2D52F6;
        }

        .github-link {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
        }

        .github-link svg {
            opacity: 0.7;
            transition: opacity 0.2s;
        }

        .github-link:hover svg {
            opacity: 1;
        }

        /* Animations */
        .fade-in {
            animation: fadeIn 0.5s ease forwards;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(8px); }
            to { opacity: 1; transform: translateY(0); }
        }

        .card { animation-delay: 0.1s; opacity: 0; }
        .footer { animation-delay: 0.2s; opacity: 0; }

        /* Console auth section */
        .console-auth-section {
            text-align: center;
            margin-bottom: 1.5rem;
        }

        .btn-console {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 0.75rem;
            width: 100%;
            padding: 1rem 1.5rem;
            font-family: 'Inter', sans-serif;
            font-size: 0.9375rem;
            font-weight: 600;
            background: var(--accent);
            color: #000;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            transition: all 0.2s ease;
            text-decoration: none;
        }

        .btn-console:hover {
            background: var(--accent-dark);
            transform: translateY(-1px);
            box-shadow: 0 4px 12px var(--accent-glow);
        }

        .btn-console:active {
            transform: translateY(0);
        }

        .btn-icon {
            width: 20px;
            height: 20px;
        }

        .console-hint {
            margin-top: 0.625rem;
            font-size: 0.75rem;
            color: var(--text-dim);
        }

        /* Divider */
        .divider {
            display: flex;
            align-items: center;
            gap: 1rem;
            margin: 1.5rem 0;
            color: var(--text-dim);
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.08em;
        }

        .divider::before,
        .divider::after {
            content: '';
            flex: 1;
            height: 1px;
            background: var(--border);
        }
    </style>
</head>
<body>
    <!-- Background elements -->
    <svg class="bg-pattern" viewBox="0 0 1000 1000">
        <circle cx="500" cy="500" r="100"/>
        <circle cx="500" cy="500" r="200"/>
        <circle cx="500" cy="500" r="300"/>
        <circle cx="500" cy="500" r="400"/>
        <circle cx="500" cy="500" r="500"/>
    </svg>
    <div class="bg-gradient"></div>

    <div class="container">
        <div class="terminal-header">notte auth login</div>

        <div class="logo-section fade-in">
            <svg class="logo" viewBox="0 0 1355.92 333" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
                <path d="M102,151L14.92,100.36c-.19-1.77.54-3.21,1.18-4.77,1.04-2.52,12.71-22.52,13.91-23.15.74-.39,1.43-.56,2.24-.3l85.74,48.85-48.97-86.61.1-2.26c5.18-.86,24.13-15.8,27.18-15,1.69.44,4.88,6.17,6.11,7.97,24.65,35.97,45.93,74.46,70.56,110.44,7.18,10.48,13.6,19.57,25.1,25.9l118.48,74.58c.89,1.04.02,1.78-.33,2.7-2.75,7.19-12.71,17.04-15.38,25.17l-1.27.16-88.57-50.03,50.03,88.57-1.25,1.75-26.15,14.53-52.63-88.85v103h-33v-101l-51.6,86.92-26.41-14.4-1.02-2.95,49.54-87.55-87.03,49.97c-.86.05-1.41-.43-2.04-.9-1.08-.82-12.63-20.96-13.65-23.36-.56-1.31-1.36-3.26-.61-4.55l87.82-51.18H0v-33h102Z"/>
                <path d="M1354.76,184.27h-149c.75,19.62,9.58,38.33,25.08,50.42,26.13,20.37,82.29,12.12,94.96-21.4l24.95,9c-5.54,17.38-23.15,34.19-39.64,41.85-75.83,35.19-145.01-26.26-136.34-105.34,12.32-112.41,174.78-108.59,181.03-2.06.54,9.25-.99,18.35-1.04,27.54ZM1323.76,159.27c2.06-54.74-69.35-75.79-102.97-35.47-8.09,9.7-14.04,22.71-14.03,35.47h117Z"/>
                <path d="M764.56,75.57c76.65-5.89,120.98,62.19,99.92,132.92-25.73,86.44-158.86,86.66-182.64-.8-17.04-62.69,13.93-126.84,82.72-132.12ZM762.54,103.55c-73.02,8.76-72.06,135.95,1.68,141.76,51.53,4.06,77.35-32.96,73.41-81.41-3.3-40.6-34.69-65.2-75.09-60.35Z"/>
                <path d="M633.76,270.27h-29v-119.5c0-24.61-20.03-46.82-44.46-48.54-42.81-3.01-56.71,27.15-58.58,64.5-1.7,33.99,1.36,69.41.04,103.54h-28V78.27h26l2.01,25.99,9.98-11.5c52.19-38.45,117.4-11.76,122,55.02v122.49Z"/>
                <path d="M957.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
                <path d="M1092.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
                <path d="M232,0c-.96,19.84-.79,39.61-.96,59.54-.04,4.21-1.33,7.98-1.02,12.95,1,16.26,12.81,30.81,29.49,32.5,19.73,2,41.25-.57,60.95-1.03,8.5-.2,17.05.23,25.54.04v34c-25.16-.15-50.36.23-75.54-.05-20.95-.23-35-1.55-51.47-16.44-34.31-31-17.81-75.49-21.03-116.06l1.05-5.45h33Z"/>
            </svg>
            <h1>Connect to Notte</h1>
            <p class="subtitle">Authenticate your CLI</p>
        </div>

        <div class="card fade-in">
            <!-- Console Sign-in Section (Primary) -->
            <div class="console-auth-section">
                <a href="{{.ConsoleAuthURL}}" class="btn-console">
                    <svg class="btn-icon" viewBox="0 0 1355.92 333" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
                        <path d="M102,151L14.92,100.36c-.19-1.77.54-3.21,1.18-4.77,1.04-2.52,12.71-22.52,13.91-23.15.74-.39,1.43-.56,2.24-.3l85.74,48.85-48.97-86.61.1-2.26c5.18-.86,24.13-15.8,27.18-15,1.69.44,4.88,6.17,6.11,7.97,24.65,35.97,45.93,74.46,70.56,110.44,7.18,10.48,13.6,19.57,25.1,25.9l118.48,74.58c.89,1.04.02,1.78-.33,2.7-2.75,7.19-12.71,17.04-15.38,25.17l-1.27.16-88.57-50.03,50.03,88.57-1.25,1.75-26.15,14.53-52.63-88.85v103h-33v-101l-51.6,86.92-26.41-14.4-1.02-2.95,49.54-87.55-87.03,49.97c-.86.05-1.41-.43-2.04-.9-1.08-.82-12.63-20.96-13.65-23.36-.56-1.31-1.36-3.26-.61-4.55l87.82-51.18H0v-33h102Z"/>
                        <path d="M232,0c-.96,19.84-.79,39.61-.96,59.54-.04,4.21-1.33,7.98-1.02,12.95,1,16.26,12.81,30.81,29.49,32.5,19.73,2,41.25-.57,60.95-1.03,8.5-.2,17.05.23,25.54.04v34c-25.16-.15-50.36.23-75.54-.05-20.95-.23-35-1.55-51.47-16.44-34.31-31-17.81-75.49-21.03-116.06l1.05-5.45h33Z"/>
                    </svg>
                    Sign in with Notte Console
                </a>
                <p class="console-hint">Recommended - automatically fetches your API key</p>
            </div>

            <!-- Divider -->
            <div class="divider">
                <span>or</span>
            </div>

            <!-- Manual API Key Section (Secondary) -->
            <form id="setupForm" autocomplete="off">
                <div class="form-group">
                    <label for="apiKey">Enter API Key manually</label>
                    <input
                        type="password"
                        id="apiKey"
                        name="apiKey"
                        placeholder="notte_xxxxxxxxxxxxxxxx"
                        required
                    >
                    <div class="input-hint">
                        Get your key from <a href="https://console.notte.cc/apikeys" target="_blank">console.notte.cc/apikeys</a>
                    </div>
                </div>

                <div class="btn-group">
                    <button type="button" id="testBtn" class="btn-secondary">Test</button>
                    <button type="submit" id="submitBtn" class="btn-primary">Save & Connect</button>
                </div>

                <div id="status" class="status"></div>
            </form>
        </div>

        <div class="footer fade-in">
            <a href="https://github.com/nottelabs/notte-cli" target="_blank" class="github-link">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                </svg>
                View on GitHub
            </a>
        </div>
    </div>

    <script>
        const form = document.getElementById('setupForm');
        const testBtn = document.getElementById('testBtn');
        const submitBtn = document.getElementById('submitBtn');
        const status = document.getElementById('status');
        const csrfToken = '{{.CSRFToken}}';

        function showStatus(type, message) {
            status.className = 'status show ' + type;
            if (type === 'loading') {
                status.innerHTML = '<div class="spinner"></div><span>' + message + '</span>';
            } else {
                const icon = type === 'success' ? '&#10003;' : '&#10007;';
                status.innerHTML = '<span>' + icon + '</span><span>' + message + '</span>';
            }
        }

        function hideStatus() {
            status.className = 'status';
        }

        testBtn.addEventListener('click', async () => {
            const apiKey = document.getElementById('apiKey').value.trim();

            if (!apiKey) {
                showStatus('error', 'Please enter your API key');
                return;
            }

            testBtn.disabled = true;
            submitBtn.disabled = true;
            showStatus('loading', 'Testing connection...');

            try {
                const response = await fetch('/validate', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': csrfToken
                    },
                    body: JSON.stringify({ api_key: apiKey })
                });

                const result = await response.json();

                if (result.success) {
                    showStatus('success', 'Connected successfully');
                } else {
                    showStatus('error', result.error);
                }
            } catch (err) {
                showStatus('error', 'Connection failed: ' + err.message);
            } finally {
                testBtn.disabled = false;
                submitBtn.disabled = false;
            }
        });

        form.addEventListener('submit', async (e) => {
            e.preventDefault();

            const apiKey = document.getElementById('apiKey').value.trim();

            if (!apiKey) {
                showStatus('error', 'Please enter your API key');
                return;
            }

            testBtn.disabled = true;
            submitBtn.disabled = true;
            showStatus('loading', 'Saving credentials...');

            try {
                const response = await fetch('/submit', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-CSRF-Token': csrfToken
                    },
                    body: JSON.stringify({ api_key: apiKey })
                });

                const result = await response.json();

                if (result.success) {
                    showStatus('success', 'Credentials saved! Redirecting...');
                    setTimeout(() => {
                        window.location.href = '/success';
                    }, 800);
                } else {
                    showStatus('error', result.error);
                    testBtn.disabled = false;
                    submitBtn.disabled = false;
                }
            } catch (err) {
                showStatus('error', 'Request failed: ' + err.message);
                testBtn.disabled = false;
                submitBtn.disabled = false;
            }
        });
    </script>
</body>
</html>`

const successTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Connected - Notte CLI</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Playfair+Display:wght@500;600&family=Inter:wght@300;400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #fafafa;
            --bg-card: #ffffff;
            --bg-terminal: #18181b;
            --border: #e4e4e7;
            --text: #18181b;
            --text-muted: #71717a;
            --text-dim: #a1a1aa;
            --accent: #00E06B;
            --accent-dark: #00c45e;
            --accent-glow: rgba(0, 224, 107, 0.12);
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Inter', -apple-system, sans-serif;
            background: var(--bg);
            color: var(--text);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
            position: relative;
        }

        /* Concentric circles background */
        .bg-pattern {
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            width: 1200px;
            height: 1200px;
            pointer-events: none;
            z-index: 0;
            opacity: 0.35;
        }

        .bg-pattern circle {
            fill: none;
            stroke: #d4d4d8;
            stroke-width: 1;
        }

        .bg-gradient {
            position: fixed;
            top: -20%;
            left: 50%;
            transform: translateX(-50%);
            width: 600px;
            height: 600px;
            background: radial-gradient(circle, var(--accent-glow) 0%, transparent 70%);
            pointer-events: none;
            z-index: 0;
            animation: pulse 3s ease-in-out infinite;
        }

        @keyframes pulse {
            0%, 100% { opacity: 0.6; transform: translateX(-50%) scale(1); }
            50% { opacity: 0.9; transform: translateX(-50%) scale(1.05); }
        }

        .container {
            width: 100%;
            max-width: 520px;
            position: relative;
            z-index: 1;
            text-align: center;
        }

        .logo {
            height: 36px;
            margin: 0 auto 2rem;
            color: var(--text);
            animation: scaleIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
        }

        @keyframes scaleIn {
            from { transform: scale(0.8); opacity: 0; }
            to { transform: scale(1); opacity: 1; }
        }

        .checkmark {
            width: 64px;
            height: 64px;
            margin: 0 auto 1.5rem;
            background: var(--accent);
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            animation: scaleIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1) 0.1s both;
            box-shadow: 0 8px 32px var(--accent-glow);
        }

        .checkmark svg {
            width: 28px;
            height: 28px;
            color: #000;
        }

        h1 {
            font-family: 'Playfair Display', Georgia, serif;
            font-size: 2rem;
            font-weight: 500;
            letter-spacing: -0.02em;
            margin-bottom: 0.5rem;
            animation: fadeUp 0.4s ease 0.15s both;
        }

        .subtitle {
            color: var(--text-muted);
            font-size: 1rem;
            font-weight: 300;
            margin-bottom: 2.5rem;
            animation: fadeUp 0.4s ease 0.2s both;
        }

        @keyframes fadeUp {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }

        /* Terminal card */
        .terminal {
            background: var(--bg-terminal);
            border-radius: 12px;
            overflow: hidden;
            text-align: left;
            animation: fadeUp 0.4s ease 0.25s both;
            box-shadow:
                0 2px 8px rgba(0, 0, 0, 0.08),
                0 8px 32px rgba(0, 0, 0, 0.12);
        }

        .terminal-bar {
            background: #27272a;
            padding: 0.75rem 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .terminal-dot {
            width: 12px;
            height: 12px;
            border-radius: 50%;
        }

        .terminal-dot.red { background: #ff5f57; }
        .terminal-dot.yellow { background: #febc2e; }
        .terminal-dot.green { background: #28c840; }

        .terminal-title {
            flex: 1;
            text-align: center;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.6875rem;
            color: #52525b;
            margin-right: 36px;
        }

        .terminal-body {
            padding: 1.25rem 1.5rem;
        }

        .terminal-line {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8125rem;
            margin-bottom: 0.75rem;
            color: #e4e4e7;
        }

        .terminal-line:last-child {
            margin-bottom: 0;
        }

        .terminal-prompt {
            color: var(--accent);
            user-select: none;
        }

        .terminal-output {
            color: #a1a1aa;
            padding-left: 1rem;
            margin-top: -0.375rem;
            margin-bottom: 0.75rem;
            font-size: 0.75rem;
        }

        .terminal-cursor {
            display: inline-block;
            width: 8px;
            height: 18px;
            background: var(--accent);
            animation: cursorBlink 1.2s step-end infinite;
            margin-left: 2px;
            vertical-align: middle;
        }

        @keyframes cursorBlink {
            0%, 50% { opacity: 1; }
            50.01%, 100% { opacity: 0; }
        }

        /* Message */
        .message {
            margin-top: 2rem;
            padding: 1.25rem;
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 12px;
            animation: fadeUp 0.4s ease 0.3s both;
        }

        .message-icon {
            font-size: 1.25rem;
            margin-bottom: 0.375rem;
        }

        .message-title {
            font-weight: 500;
            font-size: 0.9375rem;
            margin-bottom: 0.25rem;
        }

        .message-text {
            font-size: 0.8125rem;
            color: var(--text-muted);
            line-height: 1.5;
        }

        .message-text code {
            font-family: 'JetBrains Mono', monospace;
            background: var(--bg);
            padding: 0.125rem 0.375rem;
            border-radius: 4px;
            font-size: 0.75rem;
            color: var(--accent-dark);
        }

        .footer {
            text-align: center;
            margin-top: 2rem;
            font-size: 0.8125rem;
            color: var(--text-dim);
            animation: fadeUp 0.4s ease 0.35s both;
        }

        .footer a {
            color: var(--text-muted);
            text-decoration: none;
            transition: color 0.2s;
        }

        .footer a:hover {
            color: #2D52F6;
        }

        .github-link {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
        }

        .github-link svg {
            opacity: 0.7;
            transition: opacity 0.2s;
        }

        .github-link:hover svg {
            opacity: 1;
        }
    </style>
</head>
<body>
    <!-- Background elements -->
    <svg class="bg-pattern" viewBox="0 0 1000 1000">
        <circle cx="500" cy="500" r="100"/>
        <circle cx="500" cy="500" r="200"/>
        <circle cx="500" cy="500" r="300"/>
        <circle cx="500" cy="500" r="400"/>
        <circle cx="500" cy="500" r="500"/>
    </svg>
    <div class="bg-gradient"></div>

    <div class="container">
        <svg class="logo" viewBox="0 0 1355.92 333" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
            <path d="M102,151L14.92,100.36c-.19-1.77.54-3.21,1.18-4.77,1.04-2.52,12.71-22.52,13.91-23.15.74-.39,1.43-.56,2.24-.3l85.74,48.85-48.97-86.61.1-2.26c5.18-.86,24.13-15.8,27.18-15,1.69.44,4.88,6.17,6.11,7.97,24.65,35.97,45.93,74.46,70.56,110.44,7.18,10.48,13.6,19.57,25.1,25.9l118.48,74.58c.89,1.04.02,1.78-.33,2.7-2.75,7.19-12.71,17.04-15.38,25.17l-1.27.16-88.57-50.03,50.03,88.57-1.25,1.75-26.15,14.53-52.63-88.85v103h-33v-101l-51.6,86.92-26.41-14.4-1.02-2.95,49.54-87.55-87.03,49.97c-.86.05-1.41-.43-2.04-.9-1.08-.82-12.63-20.96-13.65-23.36-.56-1.31-1.36-3.26-.61-4.55l87.82-51.18H0v-33h102Z"/>
            <path d="M1354.76,184.27h-149c.75,19.62,9.58,38.33,25.08,50.42,26.13,20.37,82.29,12.12,94.96-21.4l24.95,9c-5.54,17.38-23.15,34.19-39.64,41.85-75.83,35.19-145.01-26.26-136.34-105.34,12.32-112.41,174.78-108.59,181.03-2.06.54,9.25-.99,18.35-1.04,27.54ZM1323.76,159.27c2.06-54.74-69.35-75.79-102.97-35.47-8.09,9.7-14.04,22.71-14.03,35.47h117Z"/>
            <path d="M764.56,75.57c76.65-5.89,120.98,62.19,99.92,132.92-25.73,86.44-158.86,86.66-182.64-.8-17.04-62.69,13.93-126.84,82.72-132.12ZM762.54,103.55c-73.02,8.76-72.06,135.95,1.68,141.76,51.53,4.06,77.35-32.96,73.41-81.41-3.3-40.6-34.69-65.2-75.09-60.35Z"/>
            <path d="M633.76,270.27h-29v-119.5c0-24.61-20.03-46.82-44.46-48.54-42.81-3.01-56.71,27.15-58.58,64.5-1.7,33.99,1.36,69.41.04,103.54h-28V78.27h26l2.01,25.99,9.98-11.5c52.19-38.45,117.4-11.76,122,55.02v122.49Z"/>
            <path d="M957.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
            <path d="M1092.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
            <path d="M232,0c-.96,19.84-.79,39.61-.96,59.54-.04,4.21-1.33,7.98-1.02,12.95,1,16.26,12.81,30.81,29.49,32.5,19.73,2,41.25-.57,60.95-1.03,8.5-.2,17.05.23,25.54.04v34c-25.16-.15-50.36.23-75.54-.05-20.95-.23-35-1.55-51.47-16.44-34.31-31-17.81-75.49-21.03-116.06l1.05-5.45h33Z"/>
        </svg>

        <div class="checkmark">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
        </div>

        <h1>You're connected</h1>
        <p class="subtitle">Notte CLI is ready to automate</p>

        <div class="terminal">
            <div class="terminal-bar">
                <span class="terminal-dot red"></span>
                <span class="terminal-dot yellow"></span>
                <span class="terminal-dot green"></span>
                <span class="terminal-title">Terminal</span>
            </div>
            <div class="terminal-body">
                <div class="terminal-line">
                    <span class="terminal-prompt">$</span>
                    <span>notte sessions start</span>
                </div>
                <div class="terminal-output">Starting browser session...</div>
                <div class="terminal-line">
                    <span class="terminal-prompt">$</span>
                    <span>notte scrape https://example.com</span>
                </div>
                <div class="terminal-output">Scraping webpage...</div>
                <div class="terminal-line">
                    <span class="terminal-prompt">$</span>
                    <span class="terminal-cursor"></span>
                </div>
            </div>
        </div>

        <div class="message">
            <div class="message-icon">&larr;</div>
            <div class="message-title">Return to your terminal</div>
            <div class="message-text">
                You can close this window. Try <code>notte --help</code> to see all commands.
            </div>
        </div>

        <div class="footer">
            <a href="https://github.com/nottelabs/notte-cli" target="_blank" class="github-link">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                </svg>
                View on GitHub
            </a>
        </div>
    </div>

    <script>
        fetch('/complete', { method: 'POST' }).catch(() => {});
    </script>
</body>
</html>`

const callbackTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Authenticating - Notte CLI</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500&family=Playfair+Display:wght@500;600&family=Inter:wght@300;400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #fafafa;
            --bg-card: #ffffff;
            --border: #e4e4e7;
            --text: #18181b;
            --text-muted: #71717a;
            --text-dim: #a1a1aa;
            --accent: #00E06B;
            --accent-dark: #00c45e;
            --accent-glow: rgba(0, 224, 107, 0.12);
            --error: #ef4444;
            --error-glow: rgba(239, 68, 68, 0.1);
        }

        * { margin: 0; padding: 0; box-sizing: border-box; }

        body {
            font-family: 'Inter', -apple-system, sans-serif;
            background: var(--bg);
            color: var(--text);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
            position: relative;
        }

        .bg-pattern {
            position: fixed;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            width: 1200px;
            height: 1200px;
            pointer-events: none;
            z-index: 0;
            opacity: 0.35;
        }

        .bg-pattern circle {
            fill: none;
            stroke: #d4d4d8;
            stroke-width: 1;
        }

        .container {
            width: 100%;
            max-width: 480px;
            position: relative;
            z-index: 1;
            text-align: center;
        }

        .logo {
            height: 32px;
            margin: 0 auto 2rem;
            color: var(--text);
        }

        .status-icon {
            width: 64px;
            height: 64px;
            margin: 0 auto 1.5rem;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .status-icon.loading {
            background: var(--accent-glow);
            border: 2px solid var(--accent);
        }

        .status-icon.success {
            background: var(--accent);
        }

        .status-icon.error {
            background: var(--error-glow);
            border: 2px solid var(--error);
        }

        .status-icon svg {
            width: 28px;
            height: 28px;
        }

        .status-icon.loading svg { color: var(--accent); }
        .status-icon.success svg { color: #000; }
        .status-icon.error svg { color: var(--error); }

        .spinner {
            width: 28px;
            height: 28px;
            border: 3px solid var(--accent);
            border-top-color: transparent;
            border-radius: 50%;
            animation: spin 0.8s linear infinite;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }

        h1 {
            font-family: 'Playfair Display', Georgia, serif;
            font-size: 1.75rem;
            font-weight: 500;
            letter-spacing: -0.02em;
            margin-bottom: 0.5rem;
        }

        .message {
            color: var(--text-muted);
            font-size: 0.9375rem;
            line-height: 1.6;
        }

        .error-detail {
            margin-top: 1.5rem;
            padding: 1rem;
            background: var(--bg-card);
            border: 1px solid var(--border);
            border-radius: 10px;
            font-family: 'JetBrains Mono', monospace;
            font-size: 0.8125rem;
            color: var(--error);
            word-break: break-word;
            text-align: left;
        }

        .btn-try-again {
            display: inline-flex;
            align-items: center;
            justify-content: center;
            margin-top: 1.5rem;
            padding: 0.875rem 2rem;
            font-family: 'Inter', sans-serif;
            font-size: 0.875rem;
            font-weight: 500;
            background: var(--accent);
            color: #000;
            border: none;
            border-radius: 10px;
            cursor: pointer;
            transition: all 0.2s ease;
            text-decoration: none;
        }

        .btn-try-again:hover {
            background: var(--accent-dark);
            transform: translateY(-1px);
            box-shadow: 0 4px 12px var(--accent-glow);
        }

        .hidden { display: none; }
    </style>
</head>
<body>
    <svg class="bg-pattern" viewBox="0 0 1000 1000">
        <circle cx="500" cy="500" r="100"/>
        <circle cx="500" cy="500" r="200"/>
        <circle cx="500" cy="500" r="300"/>
        <circle cx="500" cy="500" r="400"/>
        <circle cx="500" cy="500" r="500"/>
    </svg>

    <div class="container">
        <svg class="logo" viewBox="0 0 1355.92 333" xmlns="http://www.w3.org/2000/svg" fill="currentColor">
            <path d="M102,151L14.92,100.36c-.19-1.77.54-3.21,1.18-4.77,1.04-2.52,12.71-22.52,13.91-23.15.74-.39,1.43-.56,2.24-.3l85.74,48.85-48.97-86.61.1-2.26c5.18-.86,24.13-15.8,27.18-15,1.69.44,4.88,6.17,6.11,7.97,24.65,35.97,45.93,74.46,70.56,110.44,7.18,10.48,13.6,19.57,25.1,25.9l118.48,74.58c.89,1.04.02,1.78-.33,2.7-2.75,7.19-12.71,17.04-15.38,25.17l-1.27.16-88.57-50.03,50.03,88.57-1.25,1.75-26.15,14.53-52.63-88.85v103h-33v-101l-51.6,86.92-26.41-14.4-1.02-2.95,49.54-87.55-87.03,49.97c-.86.05-1.41-.43-2.04-.9-1.08-.82-12.63-20.96-13.65-23.36-.56-1.31-1.36-3.26-.61-4.55l87.82-51.18H0v-33h102Z"/>
            <path d="M1354.76,184.27h-149c.75,19.62,9.58,38.33,25.08,50.42,26.13,20.37,82.29,12.12,94.96-21.4l24.95,9c-5.54,17.38-23.15,34.19-39.64,41.85-75.83,35.19-145.01-26.26-136.34-105.34,12.32-112.41,174.78-108.59,181.03-2.06.54,9.25-.99,18.35-1.04,27.54ZM1323.76,159.27c2.06-54.74-69.35-75.79-102.97-35.47-8.09,9.7-14.04,22.71-14.03,35.47h117Z"/>
            <path d="M764.56,75.57c76.65-5.89,120.98,62.19,99.92,132.92-25.73,86.44-158.86,86.66-182.64-.8-17.04-62.69,13.93-126.84,82.72-132.12ZM762.54,103.55c-73.02,8.76-72.06,135.95,1.68,141.76,51.53,4.06,77.35-32.96,73.41-81.41-3.3-40.6-34.69-65.2-75.09-60.35Z"/>
            <path d="M633.76,270.27h-29v-119.5c0-24.61-20.03-46.82-44.46-48.54-42.81-3.01-56.71,27.15-58.58,64.5-1.7,33.99,1.36,69.41.04,103.54h-28V78.27h26l2.01,25.99,9.98-11.5c52.19-38.45,117.4-11.76,122,55.02v122.49Z"/>
            <path d="M957.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
            <path d="M1092.76,24.27v54h51v27h-51v113.5c0,.86,3.42,10.63,4.14,11.86,3.62,6.12,16.39,13.64,23.36,13.64h23.5v23.5c0,1.52-11.44,3.32-13.52,3.48-26.48,2.13-52.28-7.72-62.25-33.72-1.01-2.65-4.24-12.63-4.24-14.76v-117.5h-29v-27h29V24.27h29Z"/>
            <path d="M232,0c-.96,19.84-.79,39.61-.96,59.54-.04,4.21-1.33,7.98-1.02,12.95,1,16.26,12.81,30.81,29.49,32.5,19.73,2,41.25-.57,60.95-1.03,8.5-.2,17.05.23,25.54.04v34c-25.16-.15-50.36.23-75.54-.05-20.95-.23-35-1.55-51.47-16.44-34.31-31-17.81-75.49-21.03-116.06l1.05-5.45h33Z"/>
        </svg>

        <div class="status-icon loading" id="statusIcon">
            <div class="spinner"></div>
        </div>

        <h1 id="title">Authenticating...</h1>
        <p class="message" id="message">Please wait while we verify your credentials.</p>

        <div class="error-detail hidden" id="errorDetail"></div>
        <a href="/" class="btn-try-again hidden" id="tryAgain">Try Again</a>
    </div>

    <script>
        const expectedState = '{{.ExpectedState}}';

        (async () => {
            const statusIcon = document.getElementById('statusIcon');
            const title = document.getElementById('title');
            const message = document.getElementById('message');
            const errorDetail = document.getElementById('errorDetail');
            const tryAgain = document.getElementById('tryAgain');

            function showSuccess() {
                statusIcon.className = 'status-icon success';
                statusIcon.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>';
                title.textContent = "You're connected!";
                message.textContent = 'Redirecting...';
                // Redirect to full success page
                window.location.href = '/success';
            }

            function showError(err) {
                statusIcon.className = 'status-icon error';
                statusIcon.innerHTML = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>';
                title.textContent = 'Authentication Failed';
                message.textContent = 'Something went wrong during the authentication process.';
                errorDetail.textContent = err;
                errorDetail.classList.remove('hidden');
                tryAgain.classList.remove('hidden');
            }

            // Parse fragment: #token=xxx&state=yyy
            const params = new URLSearchParams(window.location.hash.slice(1));
            const token = params.get('token');
            const state = params.get('state');

            if (!token) {
                showError('No token received from console.');
                return;
            }

            if (state !== expectedState) {
                showError('Invalid state parameter. This may be a security issue.');
                return;
            }

            try {
                const res = await fetch('/callback', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ token, state })
                });

                const data = await res.json();

                if (res.ok && data.success) {
                    showSuccess();
                } else {
                    showError(data.error || 'Server rejected the token.');
                }
            } catch (e) {
                showError('Failed to communicate with CLI server: ' + e.message);
            }
        })();
    </script>
</body>
</html>`
