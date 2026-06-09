package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var Version = "v0.0.0"

func main() {
	global := flag.NewFlagSet("keylightctl", flag.ExitOnError)
	configPath := global.String("config", "", "config file (default $HOME/.keylightctl.json)")
	printVersion := global.Bool("version", false, "print version and exit")
	global.Usage = func() { printUsage(global) }
	global.Parse(os.Args[1:])

	if *printVersion {
		fmt.Println(Version)
		return
	}

	if *configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, "cannot determine home directory:", err)
			os.Exit(1)
		}
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			configHome = home + "/.config"
		}
		*configPath = configHome + "/keylightctl/config.json"
	}

	lights, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error loading config:", err)
		os.Exit(1)
	}

	args := global.Args()
	if len(args) == 0 {
		runTUI(lights)
		return
	}

	switch args[0] {
	case "status":
		cmdStatus(lights, args[1:])
	case "on":
		cmdOn(lights, args[1:])
	case "off":
		cmdOff(lights, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(global)
		os.Exit(1)
	}
}

func loadConfig(path string) ([]LightConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg struct {
		Lights []LightConfig `json:"lights"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return cfg.Lights, nil
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintf(fs.Output(), `Usage: keylightctl [--config PATH] [--version] <command> [flags]

Commands:
  status  [-l NAME]
  on      [-b 0-100] [-t 2900-7000] [-l NAME]
  off     [-l NAME]

No command: launch interactive TUI.

Global flags:
`)
	fs.PrintDefaults()
}

func cmdStatus(lights []LightConfig, args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	lightName := fs.String("l", "", "light name")
	fs.StringVar(lightName, "light", "", "light name")
	fs.Parse(args)

	targets, ok := resolveLights(lights, *lightName)
	if !ok {
		return
	}

	ctl := newController()
	runOp(targets, ctl.getLight, "Status")
}

func cmdOn(lights []LightConfig, args []string) {
	fs := flag.NewFlagSet("on", flag.ExitOnError)
	brightness := fs.Int("b", -1, "brightness 0-100")
	fs.IntVar(brightness, "brightness", -1, "brightness 0-100")
	temperature := fs.Int("t", -1, "color temperature in Kelvin 2900-7000")
	fs.IntVar(temperature, "temperature", -1, "color temperature in Kelvin 2900-7000")
	lightName := fs.String("l", "", "light name")
	fs.StringVar(lightName, "light", "", "light name")
	fs.Parse(args)

	settings := LightDetail{On: 1}
	if *brightness != -1 {
		if err := validateBrightness(*brightness); err != nil {
			fmt.Fprintln(os.Stderr, "invalid brightness:", err)
			return
		}

		settings.Brightness = *brightness
	}

	if *temperature != -1 {
		if err := validateTemperature(*temperature); err != nil {
			fmt.Fprintln(os.Stderr, "invalid temperature:", err)
			return
		}

		settings.Temperature = kelvinToMired(*temperature)
	}

	targets, ok := resolveLights(lights, *lightName)
	if !ok {
		return
	}

	ctl := newController()
	runOp(targets, func(ip string) (*LightStatus, error) {
		return ctl.updateLight(ip, settings)
	}, "Update")
}

func cmdOff(lights []LightConfig, args []string) {
	fs := flag.NewFlagSet("off", flag.ExitOnError)
	lightName := fs.String("l", "", "light name")
	fs.StringVar(lightName, "light", "", "light name")
	fs.Parse(args)

	targets, ok := resolveLights(lights, *lightName)
	if !ok {
		return
	}

	ctl := newController()
	runOp(targets, func(ip string) (*LightStatus, error) {
		return ctl.updateLight(ip, LightDetail{On: 0})
	}, "Update")
}

func resolveLights(lights []LightConfig, name string) ([]LightConfig, bool) {
	if name == "" {
		return lights, true
	}

	for i := range lights {
		if lights[i].Name == name {
			return []LightConfig{lights[i]}, true
		}
	}

	names := make([]string, len(lights))
	for i, l := range lights {
		names[i] = l.Name
	}

	fmt.Fprintf(os.Stderr, "light %q not found; available: %s\n", name, strings.Join(names, ", "))
	return nil, false
}

type lightResult struct {
	name   string
	status *LightStatus
	err    error
}

func runOp(lights []LightConfig, op func(string) (*LightStatus, error), opName string) {
	results := make(chan lightResult, len(lights))
	done := make(chan struct{})
	var wg sync.WaitGroup

	go spinner(done)
	for _, l := range lights {
		wg.Add(1)
		go func(l LightConfig) {
			defer wg.Done()
			status, err := op(l.IP)
			results <- lightResult{name: l.Name, status: status, err: err}
		}(l)
	}

	go func() {
		wg.Wait()
		close(results)
		close(done)
	}()

	for r := range results {
		if r.err != nil {
			fmt.Printf("\r%s of light %q: error: %s\n", opName, r.name, classifyError(r.err))
			continue
		}

		for _, l := range r.status.Lights {
			fmt.Printf("\rStatus of light %q:\n", r.name)
			fmt.Printf("  Power:       %s\n", formatOnOff(l.On))
			fmt.Printf("  Brightness:  %d%%\n", l.Brightness)
			fmt.Printf("  Temperature: %dK\n", miredToKelvin(l.Temperature))
		}
	}
}

func spinner(done <-chan struct{}) {
	frames := []string{"|", "/", "-", "\\"}
	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r")
			return
		default:
			fmt.Printf("\r%s", frames[i%len(frames)])
			time.Sleep(100 * time.Millisecond)
			i++
		}
	}
}

func classifyError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "timeout while connecting"
	}

	if errors.Is(err, io.EOF) {
		return "connection closed unexpectedly"
	}

	if errors.As(err, new(*net.OpError)) {
		return "failed to connect"
	}

	return err.Error()
}

func validateBrightness(n int) error {
	if n < 0 || n > 100 {
		return fmt.Errorf("must be between 0 and 100")
	}

	return nil
}

func validateTemperature(n int) error {
	if n < 2900 || n > 7000 {
		return fmt.Errorf("must be between 2900K and 7000K")
	}

	return nil
}

func formatOnOff(on int) string {
	if on == 1 {
		return "ON"
	}

	return "OFF"
}
