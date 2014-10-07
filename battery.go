package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Watch struct {
	sysPath  string
	warn     int
	alarm    int
	shutdown int
	batterys []Battery
}

type Battery struct {
	battery        string
	energyFullPath string
	energyNowPath  string
	chargingPath   string
	energyFull     int
	energyNow      int
}

func main() {
	var err error
	var warned, alarmed bool
	var warnLimit, alarmLimit, suspendLimit int

	flag.IntVar(&warnLimit, "warn", 15, "Battery limit where you get a warn message")
	flag.IntVar(&alarmLimit, "alarm", 10, "Battery limit where you get a alarm message")
	flag.IntVar(&suspendLimit, "suspend", 5, "Battery limit where the computer get suspended")

	flag.Parse()

	w := newWatch(warnLimit, alarmLimit, suspendLimit)

	for true {
		err = nil
		lowest, isCharging := w.Update()
		switch {
		case isCharging == true:
			warned = false
			alarmed = false
		case lowest <= w.shutdown:
			cmd := exec.Command("systemctl", "suspend")
			cmd.Run()
		case lowest <= w.alarm && !alarmed:
			cmd := exec.Command("notify-send", "-i", "/usr/share/icons/gnome/48x48/status/dialog-error.png", "-u", "critical", "Battery state is critical")
			err = cmd.Run()
			alarmed = true
		case lowest <= w.warn && !warned:
			cmd := exec.Command("notify-send", "-i", "/usr/share/icons/gnome/48x48/status/dialog-warning.png", "-u", "normal", "Battery state is low")
			err = cmd.Run()
			warned = true
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while notify user: %s\n", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func readFileInt(path string) (c int, err error) {
	cStr, err := readFileString(path)
	if err != nil {
		return
	}
	c, err = strconv.Atoi(cStr)
	return
}

func readFileString(path string) (c string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()
	c = scanner.Text()
	return
}

func newBattery(sysPath string, battery string) (b Battery, err error) {
	b.battery = battery
	b.energyFullPath = fmt.Sprintf("%s%s%s", sysPath, battery, "/energy_full")
	b.energyNowPath = fmt.Sprintf("%s%s%s", sysPath, battery, "/energy_now")
	b.chargingPath = fmt.Sprintf("%s%s%s", sysPath, battery, "/status")
	return
}

func (b *Battery) State() (capacity int, isCharging bool, err error) {
	status, err := readFileString(b.chargingPath)
	if err != nil {
		return
	}
	isCharging = (strings.ToLower(status) == "charging")
	b.energyFull, err = readFileInt(b.energyFullPath)
	if err != nil {
		return
	}
	b.energyNow, err = readFileInt(b.energyNowPath)
	if err != nil {
		return
	}
	capacity = b.energyNow * 100 / b.energyFull
	return
}

func newWatch(warn int, alarm int, shutdown int) (w Watch) {
	w.alarm = alarm
	w.warn = warn
	w.shutdown = shutdown
	w.sysPath = "/sys/class/power_supply/"
	w.batterys = w.addBatterys()
	return
}

func (w *Watch) addBatterys() (b []Battery) {
	file, err := os.Open(w.sysPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading '%s': %s\n", w.sysPath, err)
		os.Exit(1)
	}
	dirNames, err := file.Readdirnames(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while reading directorys from '%s': %s\n", w.sysPath, err)
	}
	for _, dir := range dirNames {
		if dir[0:3] == "BAT" {
			nb, err := newBattery(w.sysPath, dir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error while create battery '%s': %s\n", nb.battery, err)
				continue
			}
			b = append(b, nb)
		}
	}
	return
}

func (w *Watch) Update() (lowest int, isCharging bool) {
	lowest = 100
	for _, b := range w.batterys {
		capacity, charging, err := b.State()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while reading state from '%s': %s\n", b.battery, err)
			continue
		}
		if capacity < lowest {
			lowest = capacity
		}
		if charging == true {
			isCharging = true
		}
	}
	return
}
