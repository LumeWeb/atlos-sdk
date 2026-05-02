## 0.1.2 (2026-05-02)

### Fixes

- verify postback signature against raw body
- add body size limit to HandleRequest to prevent OOM attacks

## 0.1.1 (2026-04-21)

### Features

- add /Reset endpoint to clear mock server state
- add zap logging for HTTP handlers and webhook notifications
- postback notifications now include invoice context

### Fixes

- pass production logger to mock server
- generate realistic random transaction IDs

## 0.1.0 (2026-03-27)

### Breaking Changes

- Initial release

### Features

- initialize atlos-sdk
- add wrapper methods for all client APIs and improve test coverage

### Fixes

- address PR review feedback
