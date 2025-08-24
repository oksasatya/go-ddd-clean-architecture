package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer"
	mailtpl "github.com/oksasatya/go-ddd-clean-architecture/pkg/mailer/templates"
)

func main() {
	cfg := config.Load()
	if !cfg.MailSendEnabled {
		log.Println("MAIL_SEND_ENABLED=false; email worker disabled (no real emails will be sent)")
		return
	}
	if cfg.RabbitMQURL == "" || cfg.RabbitMQEmailQueue == "" {
		log.Fatal("RabbitMQ not configured")
	}
	if cfg.MailgunDomain == "" || cfg.MailgunAPIKey == "" || cfg.MailgunSender == "" {
		log.Fatal("Mailgun not configured")
	}

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("amqp dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("amqp channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	// Prefetch biar fair dispatch
	if err := ch.Qos(16, 0, false); err != nil {
		log.Fatalf("qos: %v", err)
	}

	if _, err := ch.QueueDeclare(cfg.RabbitMQEmailQueue, true, false, false, false, nil); err != nil {
		log.Fatalf("queue declare: %v", err)
	}

	msgs, err := ch.Consume(cfg.RabbitMQEmailQueue, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("consume: %v", err)
	}

	mg := mailer.NewMailgun(cfg.MailgunDomain, cfg.MailgunAPIKey, cfg.MailgunSender)
	ctx := context.Background()
	resolver := mailtpl.IPAPIResolver{}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	go func() {
		for msg := range msgs {
			var job mailer.EmailJob
			if err := json.Unmarshal(msg.Body, &job); err != nil {
				log.Printf("bad message: %v", err)
				_ = msg.Nack(false, false)
				continue
			}

			helpers.EnsureRecipientAndEmail(&job)
			helpers.MapLegacyToUniversal(&job)

			// Localize times if we can
			helpers.LocalizeTimesIfPossible(ctx, resolver, job.Data)

			// Render
			subject := job.Subject
			text := job.Text
			html := job.HTML

			if job.Template != "" {
				if strings.EqualFold(job.Template, "universal") {
					if loc, ok := job.Data["Location"]; !ok || fmt.Sprintf("%v", loc) == "" {
						if ipVal, okIP := job.Data["IP"]; okIP {
							if g, err := resolver.Lookup(ctx, fmt.Sprintf("%v", ipVal)); err == nil {
								job.Data["Location"] = mailtpl.FormatGeo(g)
							}
						}
					}
					htmlStr, rerr := mailtpl.RenderHTML("universal", job.Data)
					if rerr != nil {
						log.Printf("render universal failed: %v", rerr)
						_ = msg.Nack(false, false)
						continue
					}
					html = htmlStr
					subject = helpers.SubjectForUniversal(job.Data)
				} else {
					s, t, h, rerr := mailtpl.Render(job.Template, job.Data)
					if rerr != nil {
						log.Printf("render %s failed: %v", job.Template, rerr)
						_ = msg.Nack(false, false)
						continue
					}
					subject, text, html = s, t, h
				}
			}

			// Send
			c, cancel := context.WithTimeout(ctx, 15*time.Second)
			if err := mg.Send(c, job.To, subject, text, html); err != nil {
				cancel()
				log.Printf("send failed: %v", err)
				_ = msg.Nack(false, true)
				continue
			}
			cancel()
			_ = msg.Ack(false)
		}
		close(done)
	}()

	log.Printf("email worker listening on queue=%s", cfg.RabbitMQEmailQueue)
	<-stop
	log.Printf("shutting down...")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
