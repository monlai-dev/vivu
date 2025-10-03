// services/mail_service.go
package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

type IMailService interface {
	SendMailToNotifyUser(
		to, subject, body, ctaText, ctaURL string,
	) error
	SendMailToResetPassword(email, token string) error
}

// SMTPConfig holds your SMTP + branding config.
type SMTPConfig struct {
	Host       string // e.g. "smtp.gmail.com"
	Port       int    // e.g. 587 (STARTTLS) or 465 (SMTPS)
	Username   string // SMTP username / login
	Password   string // SMTP password / app password
	From       string // envelope from, e.g. "no-reply@yourapp.com"
	FromName   string // display name, e.g. "Your App"
	UseSSL     bool   // true for SMTPS 465, false for STARTTLS 587
	RequireTLS bool   // if true, fail if STARTTLS not available

	AppName    string // used in footer, header
	AppBaseURL string // e.g. "https://yourapp.com"
}

type smtpMailService struct {
	cfg           SMTPConfig
	notifyTplHTML *template.Template
	resetTplHTML  *template.Template
	textTpl       *template.Template
}

func NewSMTPMailService(cfg SMTPConfig) (IMailService, error) {
	notifyHTML := template.Must(template.New("notifyHTML").Parse(baseHTMLTemplate))
	resetHTML := template.Must(template.New("resetHTML").Parse(baseHTMLTemplate))
	plainText := template.Must(template.New("plainText").Parse(plainTextTemplate))

	return &smtpMailService{
		cfg:           cfg,
		notifyTplHTML: notifyHTML,
		resetTplHTML:  resetHTML,
		textTpl:       plainText,
	}, nil
}

// ------------------- Public API -------------------

func (s *smtpMailService) SendMailToNotifyUser(
	to, subject, body, ctaText, ctaURL string,
) error {
	html, text, err := s.renderEmail(EmailData{
		Title:     subject,
		Intro:     body,
		ButtonURL: ctaURL,
		ButtonTxt: ctaText,
		AppName:   s.cfg.AppName,
		Year:      time.Now().Year(),
	})
	if err != nil {
		return err
	}
	return s.send(to, subject, html, text)
}

func (s *smtpMailService) SendMailToResetPassword(to, token string) error {
	link := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(s.cfg.AppBaseURL, "/"), url.QueryEscape(token))
	subject := "Reset your password"

	html, text, err := s.renderEmail(EmailData{
		Title:     subject,
		Intro:     "We received a request to reset your password. Click the button below to continue. If you didn’t request this, you can safely ignore this email.",
		ButtonURL: link,
		ButtonTxt: "Reset Password",
		AppName:   s.cfg.AppName,
		Year:      time.Now().Year(),
	})
	if err != nil {
		return err
	}
	return s.send(to, subject, html, text)
}

// ------------------- Rendering -------------------

type EmailData struct {
	Title     string
	Intro     string
	ButtonURL string
	ButtonTxt string
	AppName   string
	Year      int
}

const baseHTMLTemplate = `<!doctype html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>{{.Title}}</title>
  <style>
    body { 
      margin: 0; 
      padding: 0; 
      background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); 
      color: #ffffff; 
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      -webkit-font-smoothing: antialiased;
      -moz-osx-font-smoothing: grayscale;
    }
    .wrapper { 
      width: 100%; 
      padding: 40px 16px; 
      box-sizing: border-box;
    }
    .container { 
      width: 100%; 
      max-width: 600px; 
      margin: 0 auto; 
      background: #1e293b;
      border-radius: 16px; 
      overflow: hidden; 
      box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.06);
    }
    .header { 
      padding: 32px 32px 24px; 
      background: linear-gradient(180deg, #1e293b 0%, #1a2332 100%);
      border-bottom: 1px solid rgba(148, 163, 184, 0.1);
    }
    .brand { 
      font-weight: 700; 
      letter-spacing: 0.5px; 
      font-size: 22px; 
      color: #60a5fa;
      text-transform: uppercase;
      background: linear-gradient(135deg, #60a5fa 0%, #818cf8 100%);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      background-clip: text;
    }
    .hero { 
      padding: 40px 32px; 
    }
    h1 { 
      margin: 0 0 16px; 
      font-size: 28px; 
      font-weight: 700;
      color: #f1f5f9;
      line-height: 1.3;
      letter-spacing: -0.5px;
    }
    p { 
      margin: 0 0 20px; 
      line-height: 1.7; 
      color: #cbd5e1;
      font-size: 16px;
    }
    .btn-container {
      margin: 32px 0 24px;
    }
    .btn { 
      display: inline-block; 
      padding: 16px 32px; 
      background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
      color: #ffffff !important; 
      text-decoration: none; 
      border-radius: 12px; 
      font-weight: 600;
      font-size: 16px;
      box-shadow: 0 4px 14px rgba(59, 130, 246, 0.4), 0 0 0 1px rgba(59, 130, 246, 0.2);
      transition: all 0.2s ease;
      letter-spacing: 0.3px;
    }
    .btn:hover { 
      background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
      box-shadow: 0 6px 20px rgba(59, 130, 246, 0.5), 0 0 0 1px rgba(59, 130, 246, 0.3);
      transform: translateY(-1px);
    }
    .link-fallback {
      background: rgba(148, 163, 184, 0.08);
      border: 1px solid rgba(148, 163, 184, 0.15);
      border-radius: 8px;
      padding: 16px;
      margin-top: 24px;
    }
    .muted { 
      color: #94a3b8; 
      font-size: 13px;
      line-height: 1.6;
      margin: 0;
    }
    .link-text {
      color: #60a5fa;
      text-decoration: none;
      word-break: break-all;
      font-size: 13px;
      display: inline-block;
      margin-top: 8px;
    }
    .link-text:hover {
      color: #93c5fd;
      text-decoration: underline;
    }
    .footer { 
      padding: 24px 32px; 
      color: #64748b; 
      font-size: 13px; 
      text-align: center; 
      border-top: 1px solid rgba(148, 163, 184, 0.1);
      background: #1a2332;
    }
    .divider {
      height: 1px;
      background: linear-gradient(90deg, transparent 0%, rgba(148, 163, 184, 0.15) 50%, transparent 100%);
      margin: 32px 0;
    }
    
    @media (max-width: 600px) {
      .wrapper { padding: 24px 12px; }
      .header { padding: 24px 20px 20px; }
      .hero { padding: 32px 20px; }
      .footer { padding: 20px; }
      h1 { font-size: 24px; }
      .btn { padding: 14px 28px; font-size: 15px; }
    }
    
    @media (prefers-color-scheme: light) {
      body { 
        background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
        color: #0f172a; 
      }
      .container { 
        background: #ffffff;
        box-shadow: 0 20px 60px rgba(0, 0, 0, 0.08), 0 0 0 1px rgba(0, 0, 0, 0.05);
      }
      .header {
        background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
        border-bottom: 1px solid rgba(0, 0, 0, 0.06);
      }
      .brand { 
        color: #1e40af;
        background: linear-gradient(135deg, #2563eb 0%, #3b82f6 100%);
        -webkit-background-clip: text;
        -webkit-text-fill-color: transparent;
        background-clip: text;
      }
      h1 { color: #0f172a; }
      p { color: #475569; }
      .btn {
        background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
        box-shadow: 0 4px 14px rgba(59, 130, 246, 0.25), 0 0 0 1px rgba(59, 130, 246, 0.1);
      }
      .btn:hover {
        background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
        box-shadow: 0 6px 20px rgba(59, 130, 246, 0.3), 0 0 0 1px rgba(59, 130, 246, 0.15);
      }
      .link-fallback {
        background: rgba(0, 0, 0, 0.02);
        border: 1px solid rgba(0, 0, 0, 0.08);
      }
      .muted { color: #64748b; }
      .link-text { color: #2563eb; }
      .link-text:hover { color: #1d4ed8; }
      .footer { 
        color: #64748b;
        background: #f8fafc;
        border-top: 1px solid rgba(0, 0, 0, 0.06);
      }
      .divider {
        background: linear-gradient(90deg, transparent 0%, rgba(0, 0, 0, 0.08) 50%, transparent 100%);
      }
    }
  </style>
</head>
<body>
  <div class="wrapper">
    <div class="container">
      <div class="header">
        <div class="brand">{{.AppName}}</div>
      </div>
      <div class="hero">
        <h1>{{.Title}}</h1>
        <p>{{.Intro}}</p>
        {{if .ButtonURL}}
          <div class="btn-container">
            <a class="btn" href="{{.ButtonURL}}">{{.ButtonTxt}}</a>
          </div>
          <div class="link-fallback">
            <p class="muted">
              If the button doesn't work, copy and paste this link into your browser:
            </p>
            <a href="{{.ButtonURL}}" class="link-text">{{.ButtonURL}}</a>
          </div>
        {{end}}
      </div>
      <div class="footer">
        © {{.Year}} {{.AppName}}. All rights reserved.
      </div>
    </div>
  </div>
</body>
</html>`
const plainTextTemplate = `{{.Title}}

{{.Intro}}

{{if .ButtonURL}}Open this link:
{{.ButtonURL}}
{{end}}

— {{.AppName}} (c) {{.Year}}
`

func (s *smtpMailService) renderEmail(data EmailData) (html string, text string, err error) {
	var hb, tb bytes.Buffer

	// HTML
	if err = s.notifyTplHTML.Execute(&hb, data); err != nil {
		return "", "", err
	}
	// Plain text
	if err = s.textTpl.Execute(&tb, data); err != nil {
		return "", "", err
	}
	return hb.String(), tb.String(), nil
}

// ------------------- SMTP Send -------------------

func (s *smtpMailService) send(to, subject, htmlBody, textBody string) error {
	fromHeader := s.formatFromHeader()
	date := time.Now().Format(time.RFC1123Z)
	boundary := fmt.Sprintf("mixed_%d", time.Now().UnixNano())

	var msg bytes.Buffer
	write := func(format string, a ...any) { _, _ = msg.WriteString(fmt.Sprintf(format, a...)) }

	// Headers
	write("From: %s\r\n", fromHeader)
	write("To: %s\r\n", to)
	write("Subject: %s\r\n", subject)
	write("Date: %s\r\n", date)
	write("MIME-Version: 1.0\r\n")
	write("Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	write("\r\n")

	// Plaintext part
	write("--%s\r\n", boundary)
	write("Content-Type: text/plain; charset=UTF-8\r\n")
	write("Content-Transfer-Encoding: 7bit\r\n\r\n")
	write("%s\r\n\r\n", textBody)

	// HTML part
	write("--%s\r\n", boundary)
	write("Content-Type: text/html; charset=UTF-8\r\n")
	write("Content-Transfer-Encoding: 7bit\r\n\r\n")
	write("%s\r\n\r\n", htmlBody)

	// End
	write("--%s--\r\n", boundary)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)

	if s.cfg.UseSSL {
		// SMTPS (implicit TLS, usually port 465)
		tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, s.cfg.Host)
		if err != nil {
			return err
		}
		defer c.Quit()

		if err = c.Auth(auth); err != nil {
			return err
		}
		if err = c.Mail(s.cfg.From); err != nil {
			return err
		}
		if err = c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		if _, err = w.Write(msg.Bytes()); err != nil {
			return err
		}
		return w.Close()
	}

	// STARTTLS path (typically port 587)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return err
	}
	defer c.Quit()

	// Upgrade to TLS if supported
	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
		if err = c.StartTLS(tlsCfg); err != nil {
			return err
		}
	} else if s.cfg.RequireTLS {
		return fmt.Errorf("server does not support STARTTLS and RequireTLS=true")
	}

	if err = c.Auth(auth); err != nil {
		return err
	}
	if err = c.Mail(s.cfg.From); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg.Bytes()); err != nil {
		return err
	}
	return w.Close()
}

func (s *smtpMailService) formatFromHeader() string {
	name := strings.TrimSpace(s.cfg.FromName)
	if name == "" {
		return s.cfg.From
	}
	// Properly quoted display name
	return fmt.Sprintf("%s <%s>", mimeQuote(name), s.cfg.From)
}

// Basic RFC 2047 compliant encoding for non-ASCII display names (kept simple here).
func mimeQuote(s string) string {
	// For ASCII-only names, no quoting needed.
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			// Force encode if any non-ASCII (simple UTF-8 base64 word)
			enc := toBase64UTF8(s)
			return fmt.Sprintf("=?UTF-8?B?%s?=", enc)
		}
	}
	return s
}

func toBase64UTF8(s string) string {
	// lightweight local encoder to avoid importing extra pkgs
	const base64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var b bytes.Buffer
	data := []byte(s)
	for i := 0; i < len(data); i += 3 {
		var c1, c2, c3 byte
		c1 = data[i]
		var c2Present, c3Present bool
		if i+1 < len(data) {
			c2 = data[i+1]
			c2Present = true
		}
		if i+2 < len(data) {
			c3 = data[i+2]
			c3Present = true
		}
		b.WriteByte(base64[c1>>2])
		b.WriteByte(base64[((c1&0x03)<<4)|((c2&0xF0)>>4)])
		if c2Present {
			b.WriteByte(base64[((c2&0x0F)<<2)|((c3&0xC0)>>6)])
		} else {
			b.WriteByte('=')
		}
		if c3Present {
			b.WriteByte(base64[c3&0x3F])
		} else {
			b.WriteByte('=')
		}
	}
	return b.String()
}
