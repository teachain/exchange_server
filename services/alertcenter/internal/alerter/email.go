package alerter

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
	UseAWS   bool
	AWSSES   SESConfig
}

type SESConfig struct {
	Region    string
	AccessKey string
	SecretKey string
	FromARN   string
	ToARNs    []string
}

type EmailSender struct {
	config EmailConfig
	mu     sync.RWMutex
}

func NewEmailSender(cfg *viper.Viper) *EmailSender {
	es := &EmailSender{
		config: EmailConfig{
			Enabled:  cfg.GetBool("email.enabled"),
			Host:     cfg.GetString("email.host"),
			Port:     cfg.GetInt("email.port"),
			Username: cfg.GetString("email.username"),
			Password: cfg.GetString("email.password"),
			From:     cfg.GetString("email.from"),
			To:       cfg.GetStringSlice("email.to"),
			UseTLS:   cfg.GetBool("email.use_tls"),
			UseAWS:   cfg.GetBool("email.use_aws"),
		},
	}

	if es.config.UseAWS {
		es.config.AWSSES = SESConfig{
			Region:    cfg.GetString("email.aws.region"),
			AccessKey: cfg.GetString("email.aws.access_key"),
			SecretKey: cfg.GetString("email.aws.secret_key"),
			FromARN:   cfg.GetString("email.aws.from_arn"),
			ToARNs:    cfg.GetStringSlice("email.aws.to_arn"),
		}
	}

	return es
}

func (e *EmailSender) SendEmail(ctx context.Context, subject, body string) error {
	if !e.config.Enabled {
		return nil
	}

	e.mu.RLock()
	config := e.config
	e.mu.RUnlock()

	if config.UseAWS {
		return e.sendViaSES(ctx, subject, body)
	}

	return e.sendViaSMTP(ctx, subject, body)
}

func (e *EmailSender) sendViaSMTP(ctx context.Context, subject, body string) error {
	config := e.config

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	var auth smtp.Auth
	if config.Username != "" && config.Password != "" {
		auth = smtp.PlainAuth("", config.Username, config.Password, config.Host)
	}

	msg := buildEmailMessage(config.From, config.To, subject, body)

	if config.UseTLS {
		return e.sendWithTLS(addr, auth, config.From, config.To, msg)
	}

	return smtp.SendMail(addr, auth, config.From, config.To, msg)
}

func (e *EmailSender) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: strings.Split(addr, ":")[0],
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	for _, recipient := range to {
		if err = client.Rcpt(recipient); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return client.Quit()
}

func (e *EmailSender) sendViaSES(ctx context.Context, subject, body string) error {
	config := e.config

	sesClient := NewSESClient(config.AWSSES.Region, config.AWSSES.AccessKey, config.AWSSES.SecretKey)

	return sesClient.SendEmail(ctx, config.AWSSES.FromARN, config.AWSSES.ToARNs, []string{config.From}, config.To, subject, body)
}

func buildEmailMessage(from string, to []string, subject, body string) []byte {
	var sb strings.Builder

	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ",") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)

	return []byte(sb.String())
}

type SESClient struct {
	region    string
	accessKey string
	secretKey string
}

func NewSESClient(region, accessKey, secretKey string) *SESClient {
	return &SESClient{
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
	}
}

func (c *SESClient) SendEmail(ctx context.Context, fromARN string, toARNs []string, from, to []string, subject, body string) error {
	return fmt.Errorf("AWS SES integration requires AWS SDK - implement with github.com/aws/aws-sdk-go/service/ses")
}

func (e *EmailSender) UpdateConfig(cfg *viper.Viper) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config.Enabled = cfg.GetBool("email.enabled")
	e.config.Host = cfg.GetString("email.host")
	e.config.Port = cfg.GetInt("email.port")
	e.config.Username = cfg.GetString("email.username")
	e.config.Password = cfg.GetString("email.password")
	e.config.From = cfg.GetString("email.from")
	e.config.To = cfg.GetStringSlice("email.to")
	e.config.UseTLS = cfg.GetBool("email.use_tls")
	e.config.UseAWS = cfg.GetBool("email.use_aws")
}
