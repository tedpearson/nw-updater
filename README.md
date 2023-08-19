# nw-updater
Update account balances in [ynab](https://www.ynab.com/), getting current account balances from various institutions.
Current account balances are retrieved using Chrome DevTools Protocol via [chromedp](https://github.com/chromedp/chromedp). 
YNAB is updated using the YNAB API via [ynab.go](https://github.com/brunomvsouza/ynab.go).

## Currently supported institutions:
- [Fidelity](https://www.fidelity.com)
- [Fideltiy NetBenefits](https://nb.fidelity.com)
- [Schwab Retirement Plan Center](https://workplace.schwab.com)
- [Igoe Administrative Services](https://www.goigoe.com)

## Configuring
- Rename config.yaml.example to config.yaml, and add your own credentials and account names.
  - Create a file named `.passphrase` containing the passphrase to use to encrypt your passwords.
  - Use openssl to encrypt your passwords like this: `echo -n "account_password" | openssl aes-256-cbc -a -md SHA256`.
    You will enter your encryption passphrase after entering the command.
  - To create a YNAB personal access token, follow the [YNAB documentation](https://api.ynab.com/#authentication-overview).