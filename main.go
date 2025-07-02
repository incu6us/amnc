package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"text/template"
	"time"

	cli "github.com/urfave/cli/v3"
)

//go:embed alert.tmpl
var alertTemplate embed.FS

const (
	dateFormat       = "2006-01-02T15:04:05"
	templateFileName = "alert.tmpl"
)

func main() {
	cmd := cli.Command{
		Name:        "create-alert",
		Aliases:     []string{"c"},
		Usage:       "Create test alert for Alert Manager",
		Description: "This command creates a popup alert for the user.",
		Flags: []cli.Flag{
			&cli.DurationFlag{
				Name:        "alert-duration",
				Aliases:     []string{"d"},
				Value:       time.Minute,
				DefaultText: "1m",
			},
			&cli.StringFlag{
				Name:        "alert-manager-address",
				Aliases:     []string{"address"},
				Usage:       "The URL of the alert manager service",
				Value:       "localhost:9093",
				DefaultText: "localhost:9093",
				Required:    true,
			},
			&cli.BoolFlag{
				Name:    "use-tls",
				Aliases: []string{"tls"},
				Usage:   "Use TLS for the connection to the alert manager",
			},
			&cli.StringMapFlag{
				Name:    "labels",
				Aliases: []string{"l"},
				Usage:   "Labels to attach to the alert in the format key=value. Can be specified multiple times.",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Enable verbose output",
			},
		},
		Action: func(ctx context.Context, command *cli.Command) error {
			startTime := time.Now().UTC()
			endTime := startTime.Add(command.Duration("alert-duration"))

			body, err := prepareBody(command.StringMap("labels"), startTime, endTime)
			if err != nil {
				return err
			}

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPost,
				prepareAlertManagerURL(command.String("alert-manager-address"), command.Bool("use-tls")),
				bytes.NewBufferString(body),
			)
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{
				Timeout: 5 * time.Second,
			}

			resp, err := client.Do(req)
			if err != nil {
				return err
			}

			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				responseBody, err := io.ReadAll(resp.Body)
				if err != nil {
					return cli.Exit("Failed to create alert: "+resp.Status+" "+err.Error(), 1)
				}
				return cli.Exit("Failed to create alert: "+resp.Status+" "+string(responseBody), 1)
			}

			if command.Bool("verbose") {
				fmt.Printf("alert created with body: %s\n", body)
			}

			return nil
		},
	}

	err := cmd.Run(context.Background(), os.Args)
	if err != nil {
		slog.Error("Error running command", "error", err)
	}
}

func prepareAlertManagerURL(address string, useTLS bool) string {
	if useTLS {
		return "https://" + address + "/api/v2/alerts"
	}
	return "http://" + address + "/api/v2/alerts"
}

func prepareBody(labels map[string]string, startTime, endTime time.Time) (string, error) {
	tpl, err := template.New(templateFileName).ParseFS(alertTemplate, templateFileName)
	if err != nil {
		return "", err
	}
	params := TemplateParams{
		Labels:    labels,
		StartDate: startTime.Format(dateFormat),
		EndDate:   endTime.Format(dateFormat),
	}

	buf := &bytes.Buffer{}
	err = tpl.Execute(buf, params)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

type TemplateParams struct {
	Labels    map[string]string
	StartDate string
	EndDate   string
}
