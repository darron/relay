package mailgun

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	smtp "github.com/emersion/go-smtp"
	mg "github.com/mailgun/mailgun-go/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ smtp.Backend = &Backend{}
var _ smtp.User = &User{}

// Backend type
type Backend struct {
	Domain                 string
	privateKey             string
	metricsMailgunMessages *prometheus.CounterVec
}

// User type
type User struct {
	mailgunClient          mg.Mailgun
	metricsMailgunMessages *prometheus.CounterVec
}

// NewBackend returns new instance of backend
func NewBackend(domain, privateKey string) (smtp.Backend, error) {
	if domain == "" || privateKey == "" {
		return nil, fmt.Errorf("domain, privateKey must not be empty")
	}

	b := &Backend{
		Domain:     domain,
		privateKey: privateKey,
		metricsMailgunMessages: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mailgun_messages",
				Help: "A counter for messages sent",
			},
			[]string{"status"},
		),
	}

	prometheus.MustRegister(b.metricsMailgunMessages)

	return b, nil
}

// Login is used to authenticate the user
// In relay there's no need for that at the moment
func (b *Backend) Login(username, password string) (smtp.User, error) {
	return &User{
		mailgunClient:          mg.NewMailgun(b.Domain, b.privateKey),
		metricsMailgunMessages: b.metricsMailgunMessages,
	}, nil
}

// AnonymousLogin returns anonymouse user object
func (b *Backend) AnonymousLogin() (smtp.User, error) {
	return &User{
		mailgunClient:          mg.NewMailgun(b.Domain, b.privateKey),
		metricsMailgunMessages: b.metricsMailgunMessages,
	}, nil
}

func (b *Backend) ListenAndServeMetrics(addr string) error {
	s := http.Server{
		Addr:    addr,
		Handler: promhttp.Handler(),
	}
	return s.ListenAndServe()
}

// Send will send email synchronously via Mailgun service
func (u *User) Send(from string, to []string, r io.Reader) error {
	for _, recipient := range to {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		message := u.mailgunClient.NewMIMEMessage(ioutil.NopCloser(r), recipient)
		resp, id, err := u.mailgunClient.Send(ctx, message)
		if err != nil {
			u.metricsMailgunMessages.WithLabelValues("fail").Inc()
			return err
		}
		u.metricsMailgunMessages.WithLabelValues("success").Inc()
		log.Printf("ID: %s Resp: %s", id, resp)
	}
	return nil
}

// Logout is called after all operations are complete within the session
// Here in relay there's no need to implement anything special for that
func (u *User) Logout() error {
	return nil
}
