package main

import (
	"errors"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"sync"
)

type PingRequest struct {
	CommandType string `json:"type"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Data        Data   `json:"data"`
}

type TraceRequest struct {
	CommandType string                 `json:"type"`
	Source      string                 `json:"source"`
	Target      string                 `json:"target"`
	Data        map[string]interface{} `json:"data"`
}

type Data struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Jitter float64 `json:"jitter"`
	Loss   float64 `json:"loss"`
}

func (request TraceRequest) Send(con *websocket.Conn, mutex *sync.Mutex) error {
	err := request.CheckConnection(con)

	if err != nil {
		return err
	}

	logrus.Info("Writing Object to WebSocket")
	mutex.Lock()
	defer mutex.Unlock()
	err = con.WriteJSON(request)

	return err
}

func (request TraceRequest) CheckConnection(con *websocket.Conn) error {
	logrus.Info("Checking Connection To WebSocket")
	if con != nil {
		return nil
	}

	return errors.New("connection is down")
}

func (request PingRequest) Send(con *websocket.Conn, mutex *sync.Mutex) error {
	err := request.CheckConnection(con)

	if err != nil {
		return err
	}

	logrus.Info("Writing Object to WebSocket")
	mutex.Lock()
	defer mutex.Unlock()
	err = con.WriteJSON(request)

	return err
}

func (request PingRequest) CheckConnection(con *websocket.Conn) error {
	logrus.Info("Checking Connection To WebSocket")
	if con != nil {
		return nil
	}

	return errors.New("connection is down")
}
