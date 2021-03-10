package main

type Config struct {
	Hostname   string `json:"hostname"`
	TargetsUrl string `json:"targets_url"`
	WebSocket  string `json:"web_socket"`
	LogLevel   string `json:"log_level"`
}
