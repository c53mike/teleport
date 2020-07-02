/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apps

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gravitational/oxy/forward"
	"github.com/gravitational/oxy/testutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

type HandlerConfig struct {
	AuthClient  auth.ClientI
	ProxyClient reversetunnel.Server
	//Next        http.Handler
}

func (c *HandlerConfig) CheckAndSetDefaults() error {
	if c.AuthClient == nil {
		return trace.BadParameter("missing auth client")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("missing proxy client")
	}
	//if c.Next == nil {
	//	return trace.BadParameter("missing next http.Handler")
	//}

	return nil
}

type Handler struct {
	*HandlerConfig
}

func NewHandler(config *HandlerConfig) (*Handler, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		HandlerConfig: config,
	}, nil
}

type checker interface {
	CheckAccessToApp(services.App, *http.Request) error
}

// nameFromRequest extracts the application name from the "Host" header of
// the request.
func nameFromRequest(r *http.Request) (string, error) {
	requestedHost, err := utils.Host(r.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}

	parts := strings.FieldsFunc(requestedHost, func(c rune) bool {
		return c == '.'
	})
	if len(parts) == 0 {
		return "", trace.BadParameter("invalid host header: %v", requestedHost)
	}

	return parts[0], nil
}

// TODO: Add support for Trusted Clusters.
func (a *Handler) IsApp(r *http.Request) (services.App, error) {
	name, err := nameFromRequest(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := h.AuthClient.GetApp(r.Context(), defaults.Namespace, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return app, nil
}

type session struct {
}

func (s *session) Identity() (*tlsca.Identity, error) {
}

func (s *session) RoleSet() (services.RoleSet, error) {
}

func (s *session) ClusterName() (string, error) {
}

func (s *session) ClusterClient(reversetunnel.RemoteSite, error) {

}

func extractSessionCookie(r *http.Request) (*SessionCookie, error) {
	cookie, err := r.Cookie("app_session")
	if err != nil || (cookie != nil && cookie.Value == "") {
		if err != nil {
			logger.Warn(err)
		}
		return nil, trace.AccessDenied(missingCookieMsg)
	}

	d, err := DecodeCookie(cookie.Value)
	if err != nil {
		logger.Warningf("failed to decode cookie: %v", err)
		return nil, trace.AccessDenied("failed to decode cookie")
	}

}

func (h *Handler) ValidateSession(w http.ResponseWriter, r *http.Request) (*appContext, error) {
	cookie, err := exactSessionCookie(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Try and find a matching app session (services.WebSession) for this user
	// in the backend, if a session is not found, re-direct the user to the login
	// screen with a re-direct back to this application.
	s, err := h.AuthClient.GetSession(&services.SessionRequest{
		Type:       services.AppSession,
		User:       cookie.User,
		ParentHash: cookie.ParentHash,
		SessionID:  cookie.SessionID,
	})
	if err != nil {
		// TODO!
		http.Redirect(w, r, "/login&redir=https://appName", 302)
		return
	}

	//// Verify with @alex-kovoy that it's okay that bearer token is false. This
	//// appears to make sense because the bearer token is injected client side
	//// and that's not possible for AAP.
	//ctx, err := h.handler.AuthenticateRequest(w, r, false)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}

	//// Attach certificates (x509 and SSH) to *http.Request.
	//_, cert, err := ctx.GetCertificates()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r := r.WithContext(context.WithValue(r.Context(), "identity", identity))

	//// Attach services.RoleSet to *http.Request.
	//checker, err := ctx.GetRoleSet()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r = r.WithContext(context.WithValue(r.Context(), "checker", checker))

	//// Attach services.App requested to the *http.Request.
	//r = r.WithContext(context.WithValue(r.Context(), "app", app))

	//// Attach the cluster API to the request as well.
	//// TODO: Attach trusted cluster site if trusted cluster requested.
	//clusterName, err := h.appsHandler.AuthClient.GetDomainName()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//}
	//clusterClient, err := h.handler.cfg.Proxy.GetSite(clusterName)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r = r.WithContext(context.WithValue(r.Context(), "clusterName", clusterName))
	//r = r.WithContext(context.WithValue(r.Context(), "clusterClient", clusterClient))

	//// Pass the request along to the apps handler.
	//h.appsHandler.ServeHTTP(w, r)
	//return

}

// ServeHTTP will try and find the proxied application that the caller is
// requesting. If any error occurs or the application is not found, the
// request is passed to the next handler (which would be the Web UI).
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, err := h.ValidateSession(w, r)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Extract the services.App the client requested.
	app, ok := r.Context().Value("app").(services.App)
	if !ok {
		http.Error(w, "internal service error", 500)
		return
	}

	// Extract the services.RoleSet the client requested.
	checker, ok := r.Context().Value("checker").(checker)
	if !ok {
		http.Error(w, "internal service error", 500)
		return
	}
	err := checker.CheckAccessToApp(app, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("access to app %v denied", app.GetName()), 401)
		return
	}
	fmt.Printf("--> checker.CheckAccessToApp: %v.\n", err)

	// Extract the reversetunnel.RemoteSite the client requested.
	clusterClient, ok := r.Context().Value("clusterClient").(reversetunnel.RemoteSite)
	if !ok {
		fmt.Printf("--> clusterClient: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}

	// Get a net.Conn over the reverse tunnel connection.
	conn, err := clusterClient.Dial(reversetunnel.DialParams{
		ServerID: strings.Join([]string{app.GetHostUUID(), clusterClient.GetName()}, "."),
		ConnType: services.AppTunnel,
	})
	if err != nil {
		// TODO: This should say something else, like application not available to
		// the user and log the actual reason the application was down for the admin.
		// connection rejected: dial tcp 127.0.0.1:8081: connect: connection refused.
		fmt.Printf("--> Dial: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}

	clusterName, ok := r.Context().Value("clusterName").(string)
	if !ok {
		http.Error(w, "internal service error", 500)
		return
	}

	ca, err := h.AuthClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		fmt.Printf("--> ca: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}

	identity, ok := r.Context().Value("identity").(*tlsca.Identity)
	if !ok {
		fmt.Printf("--> identity: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}

	token, err := ca.JWTKeyPair()
	if err != nil {
		fmt.Printf("--> get jwt: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}
	signedToken, err := token.Sign(&jwt.SignParams{
		Email: identity.Username,
	})
	if err != nil {
		fmt.Printf("--> get signed token: %v.\n", err)
		http.Error(w, "internal service error", 500)
		return
	}

	// Forward the request over the net.Conn to the host running the application within the cluster.
	roundTripper := forward.RoundTripper(newCustomTransport(conn))
	fwd, _ := forward.New(roundTripper, forward.Rewriter(&rewriter{signedToken: signedToken}))

	//nu, _ := url.Parse(r.URL.String())
	//nu.Scheme = "https"
	//nu.Host = "rusty-gitlab.gravitational.io"
	r.URL = testutils.ParseURI("http://localhost:8081")

	// let us forward this request to another server
	//r.URL = testutils.ParseURI("https://rusty-gitlab.gravitational.io")
	//r.URL = testutils.ParseURI("localhost:8080")

	fwd.ServeHTTP(w, r)

	//w.Header().Set("Content-Type", "text/plain")
	//fmt.Fprintf(w, "Welcome to %v.", matchedApp.GetName())
}

// TODO: Strip Teleport cookies.
type rewriter struct {
	signedToken string
}

func (r *rewriter) Rewrite(req *http.Request) {
	req.Header.Add("x-teleport-jwt-assertion", r.signedToken)
	req.Header.Add("Cf-access-token", r.signedToken)

	// Wipe out any existing cookies and skip over any Teleport ones.
	req.Header.Del("Cookie")
	for _, cookie := range req.Cookies() {
		if cookie.Name == "session" {
			continue
		}
		req.AddCookie(cookie)
	}
}

type customTransport struct {
	conn net.Conn
}

func newCustomTransport(conn net.Conn) *customTransport {
	return &customTransport{
		conn: conn,
	}
}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tr := &http.Transport{
		Dial: func(network string, addr string) (net.Conn, error) {
			return t.conn, nil
		},
	}

	resp, err := tr.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	//if resp.StatusCode == 302 {
	//	u, _ := url.Parse(resp.Header.Get("Location"))
	//	u.Host = "gitlab.proxy.example.com:8080"
	//	//u.Host = "gitlab.proxy.example.com:3080"
	//	u.Scheme = "http"
	//	resp.Header.Set("Location", u.String())
	//	fmt.Printf("--> new resp: %v.\n", u.String())

	//	origCookie := resp.Header.Get("Set-Cookie")
	//	newCookie := strings.Replace(origCookie, ".gravitational.io", ".proxy.example.com", 1)
	//	newCookie = strings.Replace(newCookie, "secure", "", 1)
	//	newCookie = strings.Replace(newCookie, "HttpOnly", "", 1)
	//	resp.Header.Set("Set-Cookie", newCookie)

	//	//for _, v := range resp.Cookies() {
	//	//	v.Domain = ".proxy.example.com"
	//	//}

	//	bb, _ := httputil.DumpResponse(resp, false)
	//	fmt.Printf("--> response: %v.\n", string(bb))

	//	return resp, nil

	//}

	return resp, nil
}