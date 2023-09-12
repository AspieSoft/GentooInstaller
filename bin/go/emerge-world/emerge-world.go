package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/host"
)

// var highTemp uint = 62
// var lowTemp uint = 46

var highTemp uint = 64
var lowTemp uint = 56

func main(){
	if temp := cpuTemp(); temp > lowTemp {
		fmt.Println("CPU Too Hot!")
		fmt.Println("Waiting for it to cool down...")
		fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \r")
		for {
			time.Sleep(10 * time.Second)
			temp := cpuTemp()
			fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \r")
			if temp <= lowTemp {
				break
			}
		}
		fmt.Println("\nCPU Temperature Stable!")
	}
	fmt.Println("Installing @world...")

	root, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}

	var cmd *exec.Cmd
	if len(os.Args) > 1 && (os.Args[1] == "-r" || os.Args[1] == "--resume") {
		fmt.Println("resuming...")
		cmd = exec.Command(`emerge`, `--resume`, `--quiet`)
	}else{
		cmd = exec.Command(`emerge`, `--update`, `--deep`, `--newuse`, `--quiet`, `@world`)
	}
	cmd.Dir = root

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	cmd.Start()

	sleepMode := false
	go func(){
		for {
			time.Sleep(10 * time.Second)

			temp := cpuTemp()
			if !sleepMode && temp >= highTemp {
				sleepMode = true
				time.Sleep(5 * time.Second)
				cmd.Process.Signal(os.Kill)
				fmt.Println("CPU Too Hot!")
				fmt.Println("pausing...")
				fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \r")
			}else if sleepMode && temp <= lowTemp {
				fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \n")
				fmt.Println("CPU Temperature Stable!")
				fmt.Println("resuming...")
				cmd = exec.Command(`emerge`, `--resume`, `--quiet`)
				cmd.Dir = root
				stdout, err = cmd.StdoutPipe()
				if err != nil {
					panic(err)
				}
				cmd.Start()
				time.Sleep(5 * time.Second)
				sleepMode = false
			}
		}
	}()

	for {
		time.Sleep(5 * time.Second)

		for sleepMode {
			time.Sleep(10 * time.Second)
			fmt.Print("CPU Temp:", strconv.Itoa(int(cpuTemp()))+"°C", "          \r")
		}

		b := make([]byte, 1024)
		_, err := stdout.Read(b)
		if err == io.EOF && !sleepMode {
			time.Sleep(1 * time.Second)
			if sleepMode {
				continue
			}
			break
		}

		if b[0] != 0 {
			fmt.Print(string(b))
		}
	}

	cmd.Wait()

	fmt.Println("Installation Finished!")

	if temp := cpuTemp(); temp > lowTemp {
		fmt.Println("CPU Too Hot!")
		fmt.Println("Waiting for it to cool down...")
		fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \r")
		for {
			time.Sleep(10 * time.Second)
			temp := cpuTemp()
			fmt.Print("CPU Temp:", strconv.Itoa(int(temp))+"°C", "          \r")
			if temp <= lowTemp {
				break
			}
		}
		fmt.Println("\nCPU Temperature Stable!")
	}
}

func cpuTemp() uint {
	temps, err := host.SensorsTemperatures()
	if err != nil {
		return 0
	}

	var i float64
	var temp float64
	for _, t := range temps {
		if strings.HasSuffix(t.SensorKey, "_input") {
			i++
			temp += t.Temperature
		}
	}

	temp = math.Round(temp / i)
	if temp < 0 {
		return 0
	}
	return uint(temp)
}
