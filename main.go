// Jacob Helpa - a small, friendly command-line helper.
//
// It presents a simple menu of handy little utilities. Run it with no
// arguments for the interactive menu, or pass a command directly, e.g.:
//
//	Jacob Helpa.exe time
//	Jacob Helpa.exe calc 12 * 8
//	Jacob Helpa.exe flip
package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const appName = "Jacob Helpa"
const version = "1.0.0"

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		// Direct command mode.
		runCommand(args[0], args[1:])
		return
	}
	interactiveMenu()
}

func interactiveMenu() {
	banner()
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println()
		fmt.Println("What can I help you with?")
		fmt.Println("  1) time   - show the current date & time")
		fmt.Println("  2) calc   - do quick arithmetic  (e.g. 12 * 8)")
		fmt.Println("  3) flip   - flip a coin")
		fmt.Println("  4) roll   - roll a dice (1-6)")
		fmt.Println("  5) info   - show system info")
		fmt.Println("  6) about  - about this program")
		fmt.Println("  q) quit")
		fmt.Print("\n> ")

		line, err := reader.ReadString('\n')
		if err != nil { // EOF (e.g. piped input ended)
			fmt.Println("\nGoodbye!")
			return
		}
		choice := strings.TrimSpace(line)

		switch strings.ToLower(choice) {
		case "1", "time":
			showTime()
		case "2", "calc":
			fmt.Print("Enter an expression (e.g. 12 * 8): ")
			expr, _ := reader.ReadString('\n')
			calc(strings.Fields(strings.TrimSpace(expr)))
		case "3", "flip":
			flip()
		case "4", "roll":
			roll()
		case "5", "info":
			sysInfo()
		case "6", "about":
			about()
		case "q", "quit", "exit":
			fmt.Println("Goodbye!")
			return
		case "":
			// ignore empty input
		default:
			fmt.Printf("Sorry, I don't know %q. Try again.\n", choice)
		}
	}
}

func runCommand(cmd string, rest []string) {
	switch strings.ToLower(cmd) {
	case "time":
		showTime()
	case "calc":
		calc(rest)
	case "flip":
		flip()
	case "roll":
		roll()
	case "info":
		sysInfo()
	case "about", "version", "-v", "--version":
		about()
	case "help", "-h", "--help":
		banner()
		fmt.Println("Commands: time | calc <a> <op> <b> | flip | roll | info | about | help")
	default:
		fmt.Printf("Unknown command %q. Run with 'help' for options.\n", cmd)
	}
}

func banner() {
	fmt.Printf("=== %s v%s ===\n", appName, version)
	fmt.Println("Your friendly little helper.")
}

func showTime() {
	now := time.Now()
	fmt.Println("Current time: " + now.Format("Monday, 02 Jan 2006  15:04:05"))
}

func calc(parts []string) {
	if len(parts) != 3 {
		fmt.Println("Please give exactly: <number> <op> <number>  (op is + - * or /)")
		return
	}
	a, err1 := strconv.ParseFloat(parts[0], 64)
	b, err2 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil {
		fmt.Println("Both operands must be numbers.")
		return
	}
	var result float64
	switch parts[1] {
	case "+":
		result = a + b
	case "-":
		result = a - b
	case "*", "x", "X":
		result = a * b
	case "/":
		if b == 0 {
			fmt.Println("Cannot divide by zero.")
			return
		}
		result = a / b
	default:
		fmt.Printf("Unknown operator %q. Use + - * or /.\n", parts[1])
		return
	}
	fmt.Printf("= %g\n", result)
}

func flip() {
	if rand.Intn(2) == 0 {
		fmt.Println("Heads!")
	} else {
		fmt.Println("Tails!")
	}
}

func roll() {
	fmt.Printf("You rolled a %d.\n", rand.Intn(6)+1)
}

func sysInfo() {
	fmt.Printf("OS:   %s\n", runtime.GOOS)
	fmt.Printf("Arch: %s\n", runtime.GOARCH)
	fmt.Printf("CPUs: %d\n", runtime.NumCPU())
	if host, err := os.Hostname(); err == nil {
		fmt.Printf("Host: %s\n", host)
	}
}

func about() {
	fmt.Printf("%s v%s\n", appName, version)
	fmt.Println("A simple helper utility. Built with Go.")
}
