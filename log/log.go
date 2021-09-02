package log

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

const (
	logBasePath = "/tmp/log"
)

var (
	// global single instance
	instance *BfsLog
	// init single instance once
	onceInit sync.Once
)

func Instance() *BfsLog {
	onceInit.Do(func() {
		instance = &BfsLog{
			taskMap: make(map[string]*taskLog, 0),
		}
	})
	return instance
}

type BfsLog struct {
	lock    sync.Mutex          // mutex for unsafe map
	taskMap map[string]*taskLog // taskID and taskLog struct map
}

// LogIn BfsLog log input
func (b *BfsLog) LogIn(taskID, msg string) {
	var task *taskLog
	if t, ok := b.taskMap[taskID]; ok {
		task = t
	} else {
		// lazy load
		task = newActiveTaskLog(taskID)
		b.lock.Lock()
		defer b.lock.Unlock()
		// add new active task-log in map
		b.taskMap[taskID] = task
	}
	task.in(fmt.Sprintf("%v: %v\n", time.Now().Format("2006-01-02 15:04:05"), msg))
}

// Stop BfsLog stop one task
func (b *BfsLog) Stop(taskID string) {
	if _, ok := b.taskMap[taskID]; !ok {
		log.Println("stop task log: task not found")
		return
	}
	// send signal to task-log, close channel
	b.taskMap[taskID].closeCh <- struct{}{}
	b.taskMap[taskID].eof = true
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.taskMap, taskID)
}

// Stream BfsLog get bfs log stream
func (b *BfsLog) Stream(taskID string) chan string {
	if _, ok := b.taskMap[taskID]; !ok {
		log.Printf("get task :%v stream chan: task not found\n", taskID)
		// service broken or task is stopped, get dead stream
		// todo start multiply deadTaskLog with same taskID. resource leak
		return newDeadTaskLog(taskID).stream()
	}
	// get active stream
	return b.taskMap[taskID].stream()
}

type taskLog struct {
	path       string        // log tmp path
	closeCh    chan struct{} // channel to receive stop signal
	msgIn      chan string   // in channel for msg input
	msgOut     chan string   // out channel for msg output
	onceStream sync.Once     // once start stream out goroutine
	eof        bool
}

func newActiveTaskLog(taskID string) *taskLog {
	t := &taskLog{
		path:    fmt.Sprintf("%s/%v", logBasePath, taskID),
		closeCh: make(chan struct{}),
		msgIn:   make(chan string, 1000),
		msgOut:  make(chan string, 1000),
		eof:     false,
	}
	// start goroutine to consume msg in channel, flush it to disk
	go t.flush()
	return t
}

// DeadTaskLog task is stopped, get output channel from log file
func newDeadTaskLog(taskID string) *taskLog {
	return &taskLog{
		path:   fmt.Sprintf("%s/%v", logBasePath, taskID),
		msgOut: make(chan string, 1000),
		eof:    true,
	}
}

func (t *taskLog) in(msg string) {
	t.msgIn <- msg
}

func (t *taskLog) flush() {
	f, err := os.Create(t.path)
	if err != nil {
		return
	}
	defer f.Close()
loop:
	for {
		select {
		case msg := <-t.msgIn:
			_, err := f.WriteString(msg)
			if err != nil {
				log.Printf("flush log in file error:%v\n", err)
				continue
			}
		case <-t.closeCh:
			log.Println("stop in msg")
			break loop
		}
	}
	// write the remain msg in file
	close(t.msgIn)
	for s := range t.msgIn {
		_, err := f.WriteString(s)
		if err != nil {
			log.Printf("flush log in file error:%v\n", err)
			continue
		}
	}
}

func (t *taskLog) stream() chan string {
	// make sure only one output goroutine
	t.onceStream.Do(func() {
		// start a goroutine to read msg from file to output channel
		go t.streamOut()
	})
	return t.msgOut
}

func (t *taskLog) streamOut() {
	defer func() {
		close(t.msgOut)
	}()
	f, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer f.Close()
	buffer := bufio.NewReader(f)
	for {
		line, err := buffer.ReadString('\n')
		if err != nil && err != io.EOF {
			fmt.Println("out put channel error,", err)
			break
		}
		if t.eof && err == io.EOF {
			fmt.Println("out put channel eof")
			break
		}
		if line == "" {
			continue
		}
		t.msgOut <- line
	}
}
