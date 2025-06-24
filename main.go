package main

import (
	"context"
	"fmt"
	httpsrv "fosite-example/pkg/httpsrvv"
	"github.com/go-logr/logr"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"fosite-example/authorizationserver"
	"fosite-example/oauth2client"
	"fosite-example/resourceserver"
	goauth "golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// A valid oauth2 client (check the store) that additionally requests an OpenID Connect id token
var clientConf = goauth.Config{
	ClientID:     "my-client",
	ClientSecret: "foobar",
	RedirectURL:  "http://localhost:3846/callback",
	Scopes:       []string{"photos", "openid", "offline"},
	Endpoint: goauth.Endpoint{
		TokenURL: "http://localhost:3846/oauth2/token",
		AuthURL:  "http://localhost:3846/oauth2/auth",
	},
}

// The same thing (valid oauth2 client) but for using the client credentials grant
var appClientConf = clientcredentials.Config{
	ClientID:     "my-client",
	ClientSecret: "foobar",
	Scopes:       []string{"fosite"},
	TokenURL:     "http://localhost:3846/oauth2/token",
}

// Samle client as above, but using a different secret to demonstrate secret rotation
var appClientConfRotated = clientcredentials.Config{
	ClientID:     "my-client",
	ClientSecret: "foobaz",
	Scopes:       []string{"fosite"},
	TokenURL:     "http://localhost:3846/oauth2/token",
}

func main() {
	port := "3846"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	// create a slg.logger
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Shorten timestamp format
			if a.Key == slog.TimeKey {
				// Format: "15:04:05.000" (HH:MM:SS.mmm)
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Format("15:04:05.000"))
				}
			}
			return a
		},
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	logger.Info("Starting Server")

	router := http.NewServeMux()
	httpConfig := &httpsrv.Config{
		BindPort:     portInt,
		Tls:          false,
		DumpExchange: true,
	}

	// ### oauth2 server ###
	authorizationserver.RegisterHandlers(router) // the authorization server (fosite)

	// ### oauth2 client ###
	router.HandleFunc("/", oauth2client.HomeHandler(clientConf)) // show some links on the index

	// the following handlers are oauth2 consumers
	router.HandleFunc("/client", oauth2client.ClientEndpoint(appClientConf))            // complete a client credentials flow
	router.HandleFunc("/client-new", oauth2client.ClientEndpoint(appClientConfRotated)) // complete a client credentials flow using rotated secret
	router.HandleFunc("/owner", oauth2client.OwnerHandler(clientConf))                  // complete a resource owner password credentials flow
	router.HandleFunc("/callback", oauth2client.CallbackHandler(clientConf))            // the oauth2 callback endpoint

	// ### protected resource ###
	router.HandleFunc("/protected", resourceserver.ProtectedEndpoint(appClientConf))

	server := httpsrv.New("oidcSrv", httpConfig, router)

	ctx := logr.NewContextWithSlogLogger(context.Background(), logger)

	fmt.Println("Please open your webbrowser at http://localhost:" + port)
	_ = exec.Command("open", "http://localhost:"+port).Run()
	err = server.Start(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%error on server launchv\n", err)
		os.Exit(1)
	}

}
