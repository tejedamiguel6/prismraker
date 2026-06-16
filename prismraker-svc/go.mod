module github.com/yourname/prismraker-svc

go 1.22

require (
	github.com/gorilla/websocket v1.5.3
	github.com/yourname/go-moonraker v0.0.0
)

// Use the sibling module during local development.
replace github.com/yourname/go-moonraker => ../go-moonraker
