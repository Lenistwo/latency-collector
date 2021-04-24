package main

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"math"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	Filename                      = "config.json"
	ExecuteEvery30Seconds         = "@every 30s"
	ExecuteEvery1Min              = "@every 1m"
	NewLine                       = "\n"
	EmptyLine                     = ""
	PingCommand                   = "ping"
	ShowFailures                  = "-O"
	Count                         = "-c"
	TotalPings                    = "10"
	MtrCommand                    = "mtr"
	SecondsToKeepProbeOpen        = "-z"
	JsonOutput                    = "-j"
	MinLengthOfSuccessFullRequest = 40
	IndexOfTime                   = "time="
	ReplaceAllExpectNumbers       = "[a-zA-Z\n= ]"
	PingLinePrefix                = "PING"
	TracerouteCommand             = "traceroute"
)

var (
	wg          sync.WaitGroup
	config      Config
	ipAddresses []string
	connection  *websocket.Conn
	mutex       sync.Mutex
)

func init() {
	loadConfig()
	setupLogging()
	retrieveTargets()
	establishWebsocketConnection()
}

func main() {
	wg.Add(1)
	creatCron()
	wg.Wait()
}

func setupLogging() {
	setLoggingLevel(config.LogLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

func setLoggingLevel(level string) {
	switch level {
	case "INFO":
		logrus.SetLevel(logrus.InfoLevel)
		break
	case "WARN":
		logrus.SetLevel(logrus.WarnLevel)
		break
	case "ERROR":
		logrus.SetLevel(logrus.ErrorLevel)
		break
	case "DEBUG":
		logrus.SetLevel(logrus.DebugLevel)
		break
	case "TRACE":
		logrus.SetLevel(logrus.TraceLevel)
		break
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.Println("Setting Log Level To ", logrus.GetLevel())
}

func loadConfig() {
	logrus.Info("Started Loading Config")
	file, err := ioutil.ReadFile(Filename)
	err = json.Unmarshal(file, &config)
	checkError(err)
	logrus.Info("Loaded Config")
}

func retrieveTargets() {
	logrus.Info("Started Retrieving Targets")
	response, err := http.Get(config.TargetsUrl + config.Hostname)
	checkError(err)
	defer response.Body.Close()
	targets, err := ioutil.ReadAll(response.Body)
	err = json.Unmarshal(targets, &ipAddresses)
	checkError(err)
	logrus.Info("Ended Retrieving Targets ")
}

func establishWebsocketConnection() {
	logrus.Info("Connecting To WebSocket")
	dial, err, _ := websocket.DefaultDialer.Dial(config.WebSocket, nil)
	if err != nil {
		logrus.Info("Failed Connecting To WebSocket")
	}
	connection = dial
}

func creatCron() {
	logrus.Info("Creating Cron")
	timer := cron.New()
	_, _ = timer.AddFunc(ExecuteEvery30Seconds, ping)
	logrus.Info("Added Task ", ExecuteEvery30Seconds, PingCommand)
	_, _ = timer.AddFunc(ExecuteEvery1Min, trace)
	logrus.Info("Added Task ", ExecuteEvery1Min, TracerouteCommand)
	timer.Start()
	logrus.Info("Cron Created")
}

func ping() {
	logrus.Info("Started Pinging")
	for _, address := range ipAddresses {
		logrus.Info("Pinging ====> " + address)
		go executePingCommand(address)
	}
	logrus.Info("Ended Pining")
}

func trace() {
	logrus.Info("Started Trace")
	for _, address := range ipAddresses {
		logrus.Info("Tracing ====> " + address)
		go executeTraceCommand(address)
	}
	logrus.Info("Ended Tracing")
}

func checkError(err error) {
	if err != nil {
		logrus.Error("=================================== Error ====================================")
		logrus.Error(err)
		logrus.Error("=================================== Error ====================================")
		panic(err)
	}
}

func executeTraceCommand(ip string) {
	output, err := exec.Command(MtrCommand, SecondsToKeepProbeOpen, JsonOutput, ip).Output()
	logrus.Info("=================================== Trace ====================================")
	logrus.Info(string(output))
	logrus.Info("=================================== Trace ====================================")
	checkError(err)

	var jsonOutput map[string]interface{}
	err = json.Unmarshal(output, &jsonOutput)
	if err != nil {
		return
	}

	request := TraceRequest{
		CommandType: TracerouteCommand,
		Source:      config.Hostname,
		Target:      ip,
		Data:        jsonOutput,
	}
	logrus.Info("Created Trace Struct ")
	logrus.Info(request)

	err = request.Send(connection, &mutex)

	if err != nil {
		logrus.Warn("Sending Data Failed ", err)
		establishWebsocketConnection()
	}
}

func executePingCommand(ip string) {
	output, _ := exec.Command(PingCommand, ShowFailures, Count, TotalPings, ip).Output()
	logrus.Info("=================================== Ping ====================================")
	logrus.Info(string(output))
	logrus.Info("=================================== Ping ====================================")

	request := PingRequest{
		CommandType: PingCommand,
		Source:      config.Hostname,
		Target:      ip,
		Data:        Data{},
	}

	totalHops := 0
	lostCount := 0
	jitterCounter := 1
	var pings []float64
	var jiters []float64
	var lastPing float64

	outputLines := strings.Split(string(output), NewLine)

	logrus.Info("=================================== Ping Split ====================================")
	logrus.Info(outputLines)
	logrus.Info("=================================== Ping Split ====================================")

	for _, line := range outputLines {
		if strings.HasPrefix(line, PingLinePrefix) {
			continue
		}

		if len(line) < 2 {
			break
		}

		totalHops += 1

		time, parseError := getRequestDuration(line)

		if lastPing == 0 {
			lastPing = time
		}

		if parseError != nil {
			lostCount += 1
			continue
		}

		pings = append(pings, time)

		if jitterCounter%2 == 0 {
			jiters = append(jiters, math.Abs(lastPing-time))
			lastPing = time
		}

		jitterCounter += 1

		if request.Data.Min == 0 && request.Data.Max == 0 {
			request.Data.Min = time
			request.Data.Max = time
			continue
		}

		if request.Data.Min > time {
			request.Data.Min = time
			continue

		}

		if request.Data.Max < time {
			request.Data.Max = time
			continue
		}
	}
	request.Data.Avg = avg(pings)
	request.Data.Jitter = avg(jiters)
	request.Data.Loss = calculateLoss(float64(lostCount), float64(totalHops))
	logrus.Info("Created Ping Struct")
	logrus.Info(request)

	err := request.Send(connection, &mutex)

	if err != nil {
		logrus.Warn("Sending Data Failed", err)
		establishWebsocketConnection()
	}
}

func avg(values []float64) float64 {
	var sum float64
	for _, val := range values {
		sum += val
	}
	result := sum / float64(len(values))

	logrus.WithFields(logrus.Fields{
		"Input":  values,
		"Result": result,
	}).Info("Avg Value For")

	if math.IsNaN(result) {
		return 0
	}
	return result
}

func calculateLoss(lostCount float64, totalLength float64) float64 {
	result := lostCount / totalLength
	logrus.Info("Loss Count ", result)
	if math.IsNaN(result) {
		return 0
	}
	return result
}

func getRequestDuration(str string) (float64, error) {
	if len(str) < MinLengthOfSuccessFullRequest {
		logrus.Warn("Request Timed Out")
		return 0, errors.New("request timed out")
	}

	timeSubstring := str[strings.LastIndex(str, IndexOfTime):]
	requestDuration := regexp.MustCompile(ReplaceAllExpectNumbers).ReplaceAllString(timeSubstring, EmptyLine)
	duration, err := strconv.ParseFloat(requestDuration, 1024)
	return duration, err
}
