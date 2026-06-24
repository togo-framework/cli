package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/togo-framework/cli/internal/config"
	"github.com/togo-framework/cli/internal/ui"
)

const botLong = `Talk to your app's chat bot and AI from the CLI.

These subcommands run inside your togo project module: the CLI generates a tiny
runner that blank-imports the configured plugin + provider, boots a kernel (which
self-registers the driver from your .env), and calls into it. Install the base +
the provider you use first, e.g.:

  togo install togo-framework/bot         togo install togo-framework/bot-telegram
  togo install togo-framework/ai          togo install togo-framework/ai-openai

  # .env
  BOT_DRIVER=telegram   TELEGRAM_BOT_TOKEN=...
  AI_DRIVER=openai      OPENAI_API_KEY=...

Examples:
  togo bot send 123456789 "deploy finished ✅"   # send over the configured bot provider
  togo bot send -c 123456789 -t "hi there"
  togo bot ask "summarize today's commits"        # one-shot AI completion via the ai plugin
  togo bot chat                                    # interactive AI chat (REPL)
  togo bot send --driver discord 987... "hello"   # override the bot driver for this run`

func registerBot(root *cobra.Command) {
	c := &cobra.Command{
		Use:     "bot",
		Short:   "Send messages over your bot provider and chat with the ai plugin",
		Long:    botLong,
		GroupID: groupInfra,
	}

	send := &cobra.Command{
		Use:   "send [channel] [text...]",
		Short: "Send a message over the configured bot provider (BOT_DRIVER)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			channel, _ := cmd.Flags().GetString("channel")
			text, _ := cmd.Flags().GetString("text")
			driver, _ := cmd.Flags().GetString("driver")
			// Positional fallback: send <channel> <text...>
			if channel == "" && len(args) > 0 {
				channel, args = args[0], args[1:]
			}
			if text == "" && len(args) > 0 {
				text = strings.Join(args, " ")
			}
			if channel == "" || text == "" {
				return fmt.Errorf("usage: togo bot send <channel> <text>  (or -c/--channel and -t/--text)")
			}
			return runBotSend(proj, driver, channel, text)
		},
	}
	send.Flags().StringP("channel", "c", "", "channel / chat ID to send to")
	send.Flags().StringP("text", "t", "", "message text")
	send.Flags().String("driver", "", "bot driver override (telegram, discord, slack); default $BOT_DRIVER")

	ask := &cobra.Command{
		Use:   "ask [prompt...]",
		Short: "One-shot AI completion via the ai plugin (AI_DRIVER)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			driver, _ := cmd.Flags().GetString("ai")
			system, _ := cmd.Flags().GetString("system")
			prompt := strings.Join(args, " ")
			if strings.TrimSpace(prompt) == "" {
				return fmt.Errorf("usage: togo bot ask <prompt>")
			}
			return runBotAI(proj, driver, system, prompt, false)
		},
	}
	ask.Flags().String("ai", "", "ai driver override (openai, anthropic, …); default $AI_DRIVER")
	ask.Flags().String("system", "", "optional system prompt")

	chat := &cobra.Command{
		Use:   "chat",
		Short: "Interactive AI chat (REPL) via the ai plugin",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := loadProject(cmd)
			if err != nil {
				return err
			}
			driver, _ := cmd.Flags().GetString("ai")
			system, _ := cmd.Flags().GetString("system")
			return runBotAI(proj, driver, system, "", true)
		},
	}
	chat.Flags().String("ai", "", "ai driver override (openai, anthropic, …); default $AI_DRIVER")
	chat.Flags().String("system", "", "optional system prompt")

	c.AddCommand(send, ask, chat)
	root.AddCommand(c)
}

// resolveEnvDriver picks override > project .env value > process env value.
func resolveEnvDriver(proj *config.Project, override, key string) string {
	if override != "" {
		return strings.ToLower(strings.TrimSpace(override))
	}
	if v := dotenvValue(proj.Root, key); v != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv(key)))
}

func runBotSend(proj *config.Project, driverOverride, channel, text string) error {
	driver := resolveEnvDriver(proj, driverOverride, "BOT_DRIVER")
	if driver == "" {
		return fmt.Errorf("no bot driver — set BOT_DRIVER in .env (telegram|discord|slack) or pass --driver")
	}
	pkg := "github.com/togo-framework/bot-" + driver
	if err := ensurePluginInstalled(proj, "bot driver", driver, pkg, "bot-"+driver); err != nil {
		return err
	}
	dir, err := writeBotRunner(proj, "botsend", botSendRunnerTmpl, map[string]string{"Pkg": pkg})
	if err != nil {
		return err
	}
	ui.Step("bot send → %s  (driver: %s)", channel, driver)
	return runGoRunner(proj, dir, []string{"TOGO_BOT_CHANNEL=" + channel, "TOGO_BOT_TEXT=" + text})
}

func runBotAI(proj *config.Project, driverOverride, system, prompt string, repl bool) error {
	driver := resolveEnvDriver(proj, driverOverride, "AI_DRIVER")
	if driver == "" {
		return fmt.Errorf("no ai driver — set AI_DRIVER in .env (openai|anthropic|…) or pass --ai")
	}
	pkg := "github.com/togo-framework/ai-" + driver
	if err := ensurePluginInstalled(proj, "ai provider", driver, pkg, "ai-"+driver); err != nil {
		return err
	}
	dir, err := writeBotRunner(proj, "aichat", aiChatRunnerTmpl, map[string]string{"Pkg": pkg})
	if err != nil {
		return err
	}
	env := []string{"TOGO_AI_SYSTEM=" + system}
	if repl {
		env = append(env, "TOGO_AI_REPL=1")
		ui.Step("ai chat  (driver: %s)  — type a message, Ctrl-D to quit", driver)
	} else {
		env = append(env, "TOGO_AI_PROMPT="+prompt)
		ui.Step("ai ask  (driver: %s)", driver)
	}
	return runGoRunner(proj, dir, env)
}

// ensurePluginInstalled verifies the project's go.mod requires pkg.
func ensurePluginInstalled(proj *config.Project, kind, driver, pkg, repo string) error {
	data, err := os.ReadFile(filepath.Join(proj.Root, "go.mod"))
	if err != nil {
		return fmt.Errorf("read go.mod: %w (run this from a togo project)", err)
	}
	if !strings.Contains(string(data), pkg) {
		return fmt.Errorf("%s %q is not installed — run:\n  togo install togo-framework/%s", kind, driver, repo)
	}
	return nil
}

// runGoRunner executes a generated runner with the project env + .env + extra vars.
func runGoRunner(proj *config.Project, dir string, extra []string) error {
	rel := "./" + filepath.ToSlash(relTo(proj.Root, dir))
	c := exec.Command("go", "run", rel)
	c.Dir = proj.Root
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	c.Env = append(os.Environ(), dotenvPairs(proj.Root)...)
	c.Env = append(c.Env, extra...)
	return c.Run()
}

func writeBotRunner(proj *config.Project, name, tmpl string, data map[string]string) (string, error) {
	dir := filepath.Join(proj.Root, ".togo", "bot", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	t, err := template.New(name).Parse(tmpl)
	if err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, "main.go"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	return dir, t.Execute(f, data)
}

// dotenvValue reads a single key from <root>/.env (no export of the whole file).
func dotenvValue(root, key string) string {
	for k, v := range dotenvMap(root) {
		if k == key {
			return v
		}
	}
	return ""
}

// dotenvPairs returns the project .env as KEY=VALUE strings for a child process.
func dotenvPairs(root string) []string {
	m := dotenvMap(root)
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// dotenvMap parses <root>/.env into a map. Minimal: KEY=VALUE lines, # comments,
// optional surrounding quotes, ignores blank lines and `export ` prefixes.
func dotenvMap(root string) map[string]string {
	m := map[string]string{}
	f, err := os.Open(filepath.Join(root, ".env"))
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		v = strings.Trim(v, `"'`)
		m[k] = v
	}
	return m
}

const botSendRunnerTmpl = `// Code generated by ` + "`togo bot send`" + `. DO NOT EDIT.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/togo-framework/bot"
	"github.com/togo-framework/togo"
	_ "{{.Pkg}}"
)

func main() {
	k := togo.New()
	svc, ok := bot.FromKernel(k)
	if !ok {
		fmt.Fprintln(os.Stderr, "togo bot: no bot driver active — check BOT_DRIVER and its token in .env")
		os.Exit(1)
	}
	channel, text := os.Getenv("TOGO_BOT_CHANNEL"), os.Getenv("TOGO_BOT_TEXT")
	if err := svc.Send(context.Background(), channel, text); err != nil {
		fmt.Fprintln(os.Stderr, "togo bot:", err)
		os.Exit(1)
	}
	fmt.Printf("sent to %s via %s\n", channel, svc.Driver())
}
`

const aiChatRunnerTmpl = `// Code generated by ` + "`togo bot ask/chat`" + `. DO NOT EDIT.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/togo-framework/ai"
	"github.com/togo-framework/togo"
	_ "{{.Pkg}}"
)

func main() {
	k := togo.New()
	svc, ok := ai.FromKernel(k)
	if !ok {
		fmt.Fprintln(os.Stderr, "togo bot: no ai driver active — check AI_DRIVER and its key in .env")
		os.Exit(1)
	}
	ctx := context.Background()
	var msgs []ai.Message
	if sys := os.Getenv("TOGO_AI_SYSTEM"); sys != "" {
		msgs = append(msgs, ai.Message{Role: ai.RoleSystem, Content: sys})
	}

	ask := func(prompt string) error {
		msgs = append(msgs, ai.Message{Role: ai.RoleUser, Content: prompt})
		resp, err := svc.Chat(ctx, ai.ChatRequest{Messages: msgs})
		if err != nil {
			return err
		}
		fmt.Println(resp.Content)
		msgs = append(msgs, ai.Message{Role: ai.RoleAssistant, Content: resp.Content})
		return nil
	}

	if os.Getenv("TOGO_AI_REPL") == "1" {
		sc := bufio.NewScanner(os.Stdin)
		fmt.Print("you> ")
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				if err := ask(line); err != nil {
					fmt.Fprintln(os.Stderr, "togo bot:", err)
				}
			}
			fmt.Print("you> ")
		}
		fmt.Println()
		return
	}

	if err := ask(os.Getenv("TOGO_AI_PROMPT")); err != nil {
		fmt.Fprintln(os.Stderr, "togo bot:", err)
		os.Exit(1)
	}
}
`
