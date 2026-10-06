# Security

Solar Chicken stores your Omlet API key and Telegram tokens encrypted (AES-256-GCM, key `SECRET_KEY` in `.env`).
It is designed to run at home, reachable from your local network or a VPN (e.g. Tailscale).
Do not expose it directly to the internet without HTTPS and a strong password.

Please report vulnerabilities privately through GitHub's *Report a vulnerability* button (Security tab),
not in public issues.
