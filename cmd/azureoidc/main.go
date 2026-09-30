// Copyright 2026 Redpanda Data, Inc.
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

// Command azureoidc runs a local OIDC issuer for testing Azure workload
// identity federation against the byoc plugin without a client secret.
//
//	azureoidc keygen -out KEY
//	azureoidc publish -key KEY -issuer URL -out DIR
//	azureoidc mint -key KEY -issuer URL -subject SUB -out FILE [-lifetime D] [-every D]
//	azureoidc serve-actions -key KEY -issuer URL -subject SUB -request-token T [-url-file FILE]
//	azureoidc serve-actions -token-file FILE -request-token T [-url-file FILE]
package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/redpanda-data/terraform-provider-redpanda/internal/azureoidc"
)

const defaultAudience = "api://AzureADTokenExchange"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "azureoidc:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: azureoidc keygen|publish|mint|serve-actions [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	keyPath := fs.String("key", "", "PKCS#8 PEM signing key")
	out := fs.String("out", "", "output path")
	issuer := fs.String("issuer", "", "issuer URL, exactly as the federated credential names it")
	subject := fs.String("subject", "", "token subject, exactly as the federated credential names it")
	audience := fs.String("audience", defaultAudience, "token audience")
	lifetime := fs.Duration("lifetime", time.Hour, "token lifetime")
	every := fs.Duration("every", 0, "keep re-minting the token at this interval until interrupted")
	requestToken := fs.String("request-token", "", "bearer token the Actions endpoint stub requires")
	urlFile := fs.String("url-file", "", "file to write the Actions endpoint stub URL to")
	tokenFile := fs.String("token-file", "", "serve this already minted token instead of minting with -key")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	switch args[0] {
	case "keygen":
		if *out == "" {
			return errors.New("keygen needs -out")
		}
		pemBytes, err := azureoidc.GenerateKeyPEM()
		if err != nil {
			return err
		}
		return writeNew(*out, pemBytes)
	case "publish":
		key, err := loadKey(*keyPath)
		if err != nil {
			return err
		}
		if *issuer == "" || *out == "" {
			return errors.New("publish needs -issuer and -out")
		}
		discovery, err := azureoidc.Discovery(*issuer)
		if err != nil {
			return err
		}
		jwks, err := azureoidc.JWKS(key)
		if err != nil {
			return err
		}
		if err := writeReplace(filepath.Join(*out, ".well-known", "openid-configuration"), discovery); err != nil {
			return err
		}
		return writeReplace(filepath.Join(*out, "jwks.json"), jwks)
	case "mint":
		key, err := loadKey(*keyPath)
		if err != nil {
			return err
		}
		if *issuer == "" || *subject == "" || *out == "" {
			return errors.New("mint needs -issuer, -subject, and -out")
		}
		claims := azureoidc.Claims{Issuer: *issuer, Subject: *subject, Audience: *audience, Lifetime: *lifetime}
		if err := mintTo(key, claims, *out); err != nil {
			return err
		}
		if *every <= 0 {
			return nil
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		ticker := time.NewTicker(*every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				if err := mintTo(key, claims, *out); err != nil {
					return err
				}
			}
		}
	case "serve-actions":
		if *requestToken == "" {
			return errors.New("serve-actions needs -request-token")
		}
		if *tokenFile != "" {
			return serveActions(azureoidc.StaticActionsTokenHandler(*tokenFile, *requestToken), *urlFile, stdout)
		}
		key, err := loadKey(*keyPath)
		if err != nil {
			return err
		}
		if *issuer == "" || *subject == "" {
			return errors.New("serve-actions needs -token-file, or -key, -issuer, and -subject")
		}
		return serveActions(azureoidc.ActionsTokenHandler(key, azureoidc.Claims{Issuer: *issuer, Subject: *subject, Lifetime: *lifetime}, *requestToken), *urlFile, stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func loadKey(path string) (*rsa.PrivateKey, error) {
	if path == "" {
		return nil, errors.New("-key is required")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return azureoidc.ParseKeyPEM(data)
}

func mintTo(key *rsa.PrivateKey, claims azureoidc.Claims, path string) error {
	token, err := azureoidc.Mint(key, claims, time.Now())
	if err != nil {
		return err
	}
	return writeReplace(path, []byte(token))
}

// serveActions answers like the GitHub Actions ID-token endpoint until
// interrupted. The URL carries a query string because azurerm appends
// "&audience=..." to it.
func serveActions(handler http.Handler, urlFile string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/token?api-version=2.0", ln.Addr())
	if urlFile != "" {
		if err := writeReplace(urlFile, []byte(url)); err != nil {
			return err
		}
	}
	fmt.Fprintln(stdout, url)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func writeNew(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Clean(path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("%w (delete it first to rotate the key)", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// writeReplace writes a 0600 file through a rename so a reader re-reading a
// refreshed token file never sees it half written.
func writeReplace(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".azureoidc-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
