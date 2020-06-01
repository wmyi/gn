package glog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"sync"

	"github.com/wmyi/gn/config"
)

const (
	LevelInfo = iota
	LevelWarn
	LevelDebug
	LevelError
)

var (
	glogRef      *Glogger
	once         sync.Once
	logLevelInfo = map[string]int{
		"All":   0,
		"Info":  1,
		"Warn":  2,
		"Debug": 3,
		"Error": 4,
	}
)

func SetConfig(conf *config.Config, serverId, mode string) {
	if glogRef == nil {
		glogInit()
	}
	if conf != nil && len(serverId) > 0 {
		glogRef.logConf = &conf.LogConf
		glogRef.curServerId = serverId
		glogRef.mode = mode
	}
	glogRef.initLogFile()
}

func glogInit() {
	if glogRef == nil {
		once.Do(func() {
			glogRef = &Glogger{
				logsChan: make(chan string, 1<<10),
				IsRuning: false,
			}
			ctx, cancal := context.WithCancel(context.Background())
			go loopChanToFile(ctx, glogRef)
			glogRef.logsChanCacal = cancal
			glogRef.IsRuning = true
		})
	}
}

func isFileExist(fileName string) (error, bool, int64) {
	file, err := os.Stat(fileName)
	if err == nil {
		return nil, true, file.Size()
	}
	if os.IsNotExist(err) {
		return nil, false, 0
	}
	return err, false, 0
}

func loopChanToFile(ctx context.Context, gl *Glogger) {
	for {
		select {
		case <-ctx.Done():
			return
		case log, ok := <-gl.logsChan:
			if ok && len(log) > 0 && gl != nil {
				gl.output(log)
			}
		}

	}
}

func Done() {
	if glogRef != nil && glogRef.IsRuning {
		if glogRef.logsChanCacal != nil {
			glogRef.logsChanCacal()
		}
		if glogRef.logFile != nil {
			glogRef.logFile.Close()
		}
		glogRef.IsRuning = false
		glogRef = nil
	}
}

type Glogger struct {
	log           *Logger
	logConf       *config.LogConfig
	logFile       *os.File
	curServerId   string
	currentSize   int64
	fileName      string
	basePath      string
	mode          string
	logsChan      chan string
	logsChanCacal context.CancelFunc
	IsRuning      bool
}

func (l *Glogger) initLogFile() {
	// debug mode
	if len(l.mode) > 0 && l.mode == "debug" {
		l.log = NewLog(io.MultiWriter(os.Stdout), "[gn] ", log.Ldate|log.Ltime|log.Lshortfile)
		return
	}
	wdPath, err := os.Getwd()
	if err != nil {
		panic(err)
		return
	}
	l.basePath = path.Join(wdPath, "../../logs/")
	l.fileName = path.Join(l.basePath, l.curServerId+".log")
	// dir
	if _, exist, _ := isFileExist(l.basePath); exist == false {
		err = os.MkdirAll(l.basePath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	// log file
	if _, exist, size := isFileExist(l.fileName); exist == true {
		l.currentSize = size
	} else {
		l.currentSize = 0
	}

	l.logFile, err = os.OpenFile(l.fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		panic(err)
	}
	l.log = NewLog(io.MultiWriter(l.logFile), "[gn] ", log.Ldate|log.Lmicroseconds|log.Lshortfile)
}

func (l *Glogger) logAssembleStr(level string, isLn bool, format string, v ...interface{}) {
	configLevel, ok := logLevelInfo[l.logConf.Level]
	if !ok {
		configLevel = logLevelInfo["All"]
	}
	if inputLevel, _ := logLevelInfo[level]; configLevel > inputLevel {
		return
	}
	if l.logsChan == nil {
		return
	}
	var s string
	if isLn {
		s = fmt.Sprint(v...)
	} else {
		s = fmt.Sprintf(format, v...)
	}

	switch level {
	case "Info":
		l.logsChan <- "[Info] " + s
	case "Warn":
		l.logsChan <- "[Warn] " + s
	case "Debug":
		l.logsChan <- "[Debug] " + s
	case "Error":
		l.logsChan <- "[Error] " + s
	}

}

func (l *Glogger) output(s string) {
	if l.log == nil {
		l.log = NewLog(io.MultiWriter(os.Stdout), "[gn] ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	var sLen int = 0
	if l.log != nil {
		sLen, _ = l.log.Println(s)
	}
	l.currentSize += int64(sLen)
	// mode  debug   return
	if len(l.mode) > 0 && l.mode == "debug" {
		return
	}
	// fmt.Println(" l.currentSize  %d   maxSize   %d", l.currentSize, l.logConf.MaxLogSize)
	if l.currentSize >= l.logConf.MaxLogSize {
		l.rollingFile()
	}
}

func (l *Glogger) rollingFile() {

	fname := ""
	num := l.logConf.NumBackups - 1
	for ; num >= 1; num-- {
		fname = l.fileName + fmt.Sprintf(".%d", num)
		nfname := l.fileName + fmt.Sprintf(".%d", num+1)
		_, err := os.Lstat(fname)
		if err == nil {
			os.Rename(fname, nfname)
		}
	}

	err := os.Rename(l.fileName, fname)
	if err != nil {
		panic(err)
	}
	if err = l.logFile.Close(); err != nil {
		panic(err)
	}
	fd, err := os.OpenFile(l.fileName, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		panic(err)
	}
	l.logFile = fd
	l.log.SetOutput(io.MultiWriter(l.logFile))
	l.currentSize = 0
}

func Infof(format string, v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Info", false, format, v...)
}

func Infoln(v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Info", true, "", v...)
}

func Warnf(format string, v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Warn", false, format, v...)
}
func Warnln(v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Warn", true, "", v...)
}

func Debugf(format string, v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Debug", false, format, v...)
}

func Debugln(v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Debug", true, "", v...)
}

func Errorf(format string, v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Error", false, format, v...)
}

func Errorln(v ...interface{}) {
	if glogRef == nil {
		glogInit()
	}
	glogRef.logAssembleStr("Error", true, "", v...)
}
