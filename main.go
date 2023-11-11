package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AspieSoft/go-regex-re2/v2"
	"github.com/AspieSoft/goutil/bash"
	"github.com/AspieSoft/goutil/cputemp"
	"github.com/AspieSoft/goutil/v7"
)

const distroName = "Empoleos"

var rsyncMirror = ""
// var rsyncMirror = "rsync://127.0.0.1/gentoo-portage"
// var rsyncMirror = "rsync://mirrors.mit.edu/gentoo-portage"

const highTemp uint = 64
const lowTemp uint = 56

var geoContinent = map[string]string{
	"AF": "Africa",
	"NA": "North America",
	"OC": "Oceania",
	"AN": "Antarctica",
	"AS": "Asia",
	"EU": "Europe",
	"SA": "South America",
}

var tarballList = []string{
	"hardened-selinux-openrc",
	"selinux-openrc",
	"hardened-openrc",
	"openrc",

	"systemd-hardened-selinux",
	"hardened-selinux-systemd",
	"systemd-selinux",
	"selinux-systemd",
	"systemd-hardened",
	"hardened-systemd",
	"systemd",
}

var installServer bool
var installUSB bool

var installProgress uint
var installProgressPrintEnabled bool = true
var maxTerminalSize int = 50

var logFile *os.File

type localeInfo struct {
	timezone string
	continent string
	locale string
	keymap string
}

type cpuType struct {
	cpu string
	cpu2 string
	real string
}

func main(){
	if os.Getuid() != 0 {
		fmt.Println("Please run as root!")
		return
	}

	args := goutil.MapArgs()

	if args["chroot"] == "true" {
		installChroot()
		return
	}

	if args["chroot-pre"] == "true" {
		installChrootPrebuild()
		return
	}


	if args["server"] == "true" || args["s"] == "true" {
		installServer = true
	}

	if args["usb"] == "true" || args["u"] == "true" || args["removable"] == "true" || args["r"] == "true" {
		installUSB = true
	}

	if val, ok := args["rsync-mirror"]; ok {
		rsyncMirror = string(regex.Comp(`[^\w_\-\\\/:\.]`).RepStrLit([]byte(val), []byte{}))
	}else if val, ok := args["rsync"]; ok {
		rsyncMirror = string(regex.Comp(`[^\w_\-\\\/:\.]`).RepStrLit([]byte(val), []byte{}))
	}else if val, ok := args["mirror"]; ok {
		rsyncMirror = string(regex.Comp(`[^\w_\-\\\/:\.]`).RepStrLit([]byte(val), []byte{}))
	}

	//todo: let user select openrc or systemd
	sysinit := "openrc"
	if args["systemd"] == "true" {
		sysinit = "systemd"
	}

	//temp default
	var installDisk string

	// ask for disk
	if val, ok := args["disk"]; ok && val != "" {
		installDisk = string(regex.Comp(`[^\w_-]`).RepStrLit([]byte(args["disk"]), []byte{}))
		if installDisk == "" {
			panic(errors.New("Invalid install disk name"))
		}
	}else{
		// ask for install disk
		buf := make([]byte, 1024)
		bash.Run([]string{`lsblk`, `-e7`, `-o`, `name,size,label,fstype,mountpoint,uuid`, `-T`}, "", nil, true)
		fmt.Print("\nChoose A Disk To Wipe For The Install: ")
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			panic(err)
		}
		installDisk = string(regex.Comp(`[^\w_-]`).RepStrLit(buf[:n], []byte{}))
		if installDisk == "" {
			panic(errors.New("Invalid install disk name"))
		}

		out, err := bash.Run([]string{`lsblk`, `-lino`, `name`, `/dev/`+installDisk}, "", nil)
		if err != nil || len(out) == 0 {
			panic(err)
		}
		if i := bytes.IndexByte(out, '\n'); i != -1 {
			out = out[:i]
		}
		installDisk = string(regex.Comp(`[^\w_-]`).RepStrLit(out, []byte{}))
		if installDisk == "" {
			panic(errors.New("Invalid install disk name"))
		}

		fmt.Println("\nAre you sure you would like to wipe the following disk?")
		fmt.Println(installDisk)
		fmt.Println("This Cannot Be Undone")
		fmt.Print("(yes|No): ")
		n, err = os.Stdin.Read(buf)
		if err != nil || n == 0 {
			panic(err)
		}
		if !regex.Comp(`(?i)^yes\r?\n?$`).Match(buf[:n]) {
			return
		}
	}

	gameSize := uint64(120000)
	// gameSize := uint64(0)
	if val, ok := args["game"]; ok && val != "" {
		if size, err := strconv.ParseUint(val, 10, 64); err == nil {
			gameSize = size * 1000
		}
	}/* else{
		// ask for game size
		buf := make([]byte, 1024)
		bash.Run([]string{`lsblk`, `-e7`, `-o`, `name,size`, `/dev/`+installDisk}, "", nil, true)
		fmt.Print("\nEnter '0' for no game partition: ")
		fmt.Print("\nGame Partition (GB): ")
		n, err := os.Stdin.Read(buf)
		if err == nil && n != 0 {
			if size, err := strconv.ParseUint(string(regex.Comp(`[^\w_-]`).RepStrLit(buf[:n], []byte{})), 10, 64); err == nil {
				gameSize = size
			}
		}
	} */


	//todo: build a gui for user to select this info
	locale := getLocale()
	cpu := getCPU() //todo: have user fill in cpuType (optional)

	//todo: ask user if duel booting windows, and what disk/partition to use (also detect if windows is on the same disk as this gentoo install)
	// for same disk windows install, may need to create a temporary clone of windows, to format disk to gpt if it isnt already
	// will also need to see if there is enough space for the windows partition with the new gentoo install

	//todo: track progress of install

	os.MkdirAll("logs", 0755)
	logFileName := time.Now().Format(time.DateOnly)+"@"+time.Now().Format(time.Kitchen)
	logFileName = string(regex.Comp(`[^\w_\-@\.:]`).RepStrLit([]byte(logFileName), []byte{}))
	if file, err := os.OpenFile("logs/"+logFileName+".log", os.O_CREATE|os.O_APPEND|os.O_RDWR|os.O_SYNC, 0755); err == nil {
		logFile = file
	}

	installProgress = 0

	installGentoo(locale, cpu, installDisk, gameSize, sysinit)
}

func getLocale() localeInfo {
	timezone, err := reqUrl("https://ipapi.co/timezone")
	if err != nil {
		timezone = []byte("UTC")
	}

	continent, err := reqUrl("https://ipapi.co/continent_code")
	if err != nil {
		continent = []byte("NA")
	}
	if val, ok := geoContinent[string(continent)]; ok {
		continent = []byte(val)
	}else{
		continent = []byte("North America")
	}

	//todo: ask user for their language
	var locale string = "en_US"
	if out, err := bash.Run([]string{`locale`}, "", nil); err == nil {
		regex.Comp(`(?m)^LANG=.*$`).RepFunc(out, func(data func(int) []byte) []byte {
			buf := data(0)
			if n := bytes.IndexByte(buf, '='); n != -1 && n+1 < len(buf) {
				buf = buf[n+1:]
			}
			if n := bytes.IndexByte(buf, '.'); n != -1 && n < len(buf) {
				buf = buf[:n]
			}
			if regex.Comp(`[\w_]+`).Match(buf) {
				locale = string(buf)
			}
			return nil
		}, true)
	}

	//todo: ask user for their keyboard layout
	keymap := string(bytes.ToLower(regex.Comp(`^[a-z]+_([A-Z]+)$`).RepStr([]byte(locale), []byte("$1"))))

	return localeInfo{
		timezone: string(timezone),
		continent: string(continent),
		locale: locale,
		keymap: keymap,
	}
}

func getCPU() cpuType {
	if out, err := bash.Run([]string{`lscpu`}, "", nil); err == nil {
		var cpuType1 string
		var cpuType2 string
		var cpuTypeReal string
		if regex.Comp(`[Ii]ntel`).Match(out) {
			if regex.Comp(`64-bit`).Match(out) {
				cpuType1="amd64"
				cpuTypeReal="x86_64"
			}else{
				cpuType1="x86"
				cpuType2="i686"
				cpuTypeReal="i686"
			}
		}else if regex.Comp(`[Aa][Mm][Dd]`).Match(out) {
			cpuType1="amd64"
			cpuTypeReal="amd64"
		}else if regex.Comp(`[Aa][Rr][Mm]`).Match(out) {
			if regex.Comp(`64-bit`).Match(out) {
				cpuType1="arm64"
				cpuTypeReal="arm64"
			}else{
				cpuType1="arm"
				cpuTypeReal="arm"
			}
		}

		return cpuType{
			cpu: cpuType1,
			cpu2: cpuType2,
			real: cpuTypeReal,
		}
	}

	return cpuType{}
}

func resolveCPUType2(cpu *cpuType){
	if cpu.cpu2 == "" {
		if cpu.cpu == "intel64" || cpu.cpu == "x86_64" {
			cpu.cpu = "amd64"
			cpu.cpu2 = "i686"
			cpu.real = "x86_64"
		}else if cpu.cpu == "intel" || cpu.cpu == "x86" || cpu.cpu == "i686" {
			cpu.cpu = "x86"
			cpu.cpu2 = "i686"
			cpu.real = "i686"
		}else{
			cpu.cpu2 = cpu.cpu
		}
	}

	if cpu.real == "" {
		cpu.real = cpu.cpu2
	}
}

func findTarball(cpu cpuType, sysinit string) (name string, tarball string, err error) {
	progress := uint(5000)

	fmt.Println("Searching gentoo.org for latest tarball...")

	progressStep := progress / uint(len(tarballList))
	cProgress := installProgress

	for _, name := range tarballList {
		installProgress += progressStep

		if !strings.Contains(name, sysinit) {
			continue
		}

		if out, err := reqUrl(`https://distfiles.gentoo.org/releases/`+cpu.cpu+`/autobuilds/current-stage3-`+cpu.cpu2+`-`+name+`/latest-stage3-`+cpu.cpu2+`-`+name+`.txt`); err == nil {
			var tarball []byte
			regex.Comp(`(?m)^\s*stage3-%1-%2-([A-Za-z0-9]+)\.tar\.xz`, cpu.cpu2, name).RepFunc(out, func(data func(int) []byte) []byte {
				tarball = regex.JoinBytes(`stage3-`, cpu.cpu2, '-', name, '-', data(1), `.tar.xz`)
				return nil
			}, true)

			if tarball != nil {
				installProgress = cProgress + progress
				return name, string(tarball), nil
			}
		}
	}

	fmt.Println(errors.New("Failed to find latest tarball on gentoo.org"))

	// check cache if no tarball is found
	if files, err := os.ReadDir("cache"); err == nil {
		fmt.Println("Searching cache...")

		installProgress = cProgress

		for _, name := range tarballList {
			installProgress += progressStep

			name = string(regex.JoinBytes(`stage3-`, cpu.cpu2, '-', name))
			for _, file := range files {
				if strings.HasPrefix(file.Name(), string(tarball)) {
					installProgress = cProgress + progress
					return name, string(file.Name()), nil
				}
			}
		}
	}

	installProgress = cProgress + progress
	return "", "", io.EOF
}


func install(pkg ...string) map[string]error {
	errList := map[string]error{}

	if len(pkg) == 1 {
		_, err := bash.Run([]string{`emerge`, `--quiet`, pkg[0]}, "/", nil, true, false)
		if err != nil {
			errList[pkg[0]] = err
		}
	}else if len(pkg) > 1 {
		listSize := len(pkg)

		emergeChan := make(chan string)

		go func(){
			for{
				nextPkg := <- emergeChan
				_, err := bash.Run([]string{`emerge`, `--quiet`, nextPkg}, "/", nil, true, false)
				if err != nil {
					errList[nextPkg] = err
				}
				listSize--
			}
		}()

		size := 0
		for _, p := range pkg {
			for size > 10 {
				time.Sleep(100 * time.Millisecond)
			}
			go func(p string){
				size++
				_, err := bash.Run([]string{`emerge`, `-f`, `--quiet`, p}, "/", nil, true, false)
				if err != nil {
					errList[p] = err
				}
				emergeChan <- p
				size--
			}(p)
		}

		for listSize > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	time.Sleep(100 * time.Millisecond)
	waitToCool(false)

	if len(errList) == 0 {
		return nil
	}
	return errList
}

func installRetry(retries int, pkg ...string) map[string]error {
	retryList := []string{}
	errList := map[string]error{}

	if len(pkg) == 1 {
		_, err := bash.Run([]string{`emerge`, `--quiet`, pkg[0]}, "/", nil, true, false)
		if err != nil {
			retryList = append(retryList, pkg[0])
			errList[pkg[0]] = err
		}
	}else if len(pkg) > 1 {
		listSize := len(pkg)

		emergeChan := make(chan string)

		go func(){
			for{
				nextPkg := <- emergeChan
				_, err := bash.Run([]string{`emerge`, `--quiet`, nextPkg}, "/", nil, true, false)
				if err != nil {
					retryList = append(retryList, nextPkg)
					errList[nextPkg] = err
				}
				listSize--
			}
		}()

		size := 0
		for _, p := range pkg {
			for size > 10 {
				time.Sleep(100 * time.Millisecond)
			}
			go func(p string){
				size++
				_, err := bash.Run([]string{`emerge`, `-f`, `--quiet`, p}, "/", nil, true, false)
				if err != nil {
					retryList = append(retryList, p)
					errList[p] = err
				}
				emergeChan <- p
				size--
			}(p)
		}

		for listSize > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	time.Sleep(100 * time.Millisecond)
	waitToCool(false)

	if len(retryList) != 0 {
		if retries == 0 {
			return errList
		}
		time.Sleep(10 * time.Second)
		return installRetry(retries-1, retryList...)
	}

	return nil
}


func reqUrl(url string) ([]byte, error) {
	client := &http.Client{}
	res, err := client.Get(url)
	if err != nil {
		return []byte{}, err
	}

	if res.StatusCode != 200 {
		return []byte{}, errors.New("error status: "+strconv.Itoa(res.StatusCode))
	}

	buf := make([]byte, 1024)
	size, err := res.Body.Read(buf)
	if err != nil && err != io.EOF {
		return []byte{}, err
	}else if size == 0 {
		return []byte{}, io.EOF
	}

	return buf[:size], nil
}


func appendToFile(name string, data []byte, createWithPerm ...os.FileMode) error {
	stat, err := os.Stat(name)
	if err != nil && len(createWithPerm) != 0 {
		return os.WriteFile(name, data, createWithPerm[0])
	}else if err != nil || stat.IsDir() {
		return err
	}

	file, err := os.OpenFile(name, os.O_RDWR|os.O_APPEND, stat.Mode().Perm())
	if err != nil {
		return err
	}
	defer file.Close()

	file.Seek(0, io.SeekEnd)
	file.Write(data)
	file.Sync()

	return nil
}


//todo: have logData and logErr methods print to gui output

func logData(msg string, noln ...bool){
	installProgressPrintEnabled = false
	time.Sleep(100 * time.Millisecond)
	fmt.Print(strings.Repeat(" ", maxTerminalSize)+"\r")

	if len(noln) != 0 && noln[0] == true {
		if logFile != nil {
			logFile.Write([]byte(msg))
			logFile.Sync()
		}
		fmt.Print(msg)
	}else{
		if logFile != nil {
			logFile.Write(append([]byte(msg), '\n'))
			logFile.Sync()
		}
		fmt.Println(msg)
	}

	installProgressPrintEnabled = true
	time.Sleep(100 * time.Millisecond)
}

func logErr(err error, fatal ...bool){
	installProgressPrintEnabled = false
	time.Sleep(100 * time.Millisecond)

	fmt.Print(strings.Repeat(" ", maxTerminalSize)+"\r")

	if logFile != nil {
		logFile.Write(append([]byte(err.Error()), '\n'))
		logFile.Sync()
	}

	if len(fatal) != 0 && fatal[0] == true {
		panic(err)
	}else{
		fmt.Println(err)
	}

	installProgressPrintEnabled = true
	time.Sleep(100 * time.Millisecond)
}

func waitToCool(strict bool){
	installProgressPrintEnabled = false
	time.Sleep(100 * time.Millisecond)
	cputemp.WaitToCool(strict)
	time.Sleep(100 * time.Millisecond)
	installProgressPrintEnabled = true
}
