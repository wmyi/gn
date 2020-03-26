package glog

import (
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
	LevelPanic
	LevelFatal
)

var (
	logLevelInfo = map[string]int{
		"All":   0,
		"Info":  1,
		"Warn":  2,
		"Debug": 3,
		"Error": 4,
		"Panic": 5,
		"Fatal": 6,
	}
)

func NewLogger(conf *config.Config, serverId, mode string) *Glogger {
	if conf != nil && len(serverId) > 0 {
		log := &Glogger{
			logConf:     &conf.LogConf,
			curServerId: serverId,
			mode:        mode,
		}
		log.initLogFile()
		return log
	}
	return nil
}

func IsFileExist(fileName string) (error, bool, int64) {
	file, err := os.Stat(fileName)
	if err == nil {
		return nil, true, file.Size()
	}
	if os.IsNotExist(err) {
		return nil, false, 0
	}
	return err, false, 0
}

type Glogger struct {
	log         *Logger
	logConf     *config.LogConfig
	logFile     *os.File
	curServerId string
	currentSize int64
	fileName    string
	basePath    string
	rollingLock sync.RWMutex
	mode        string
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
	if _, exist, _ := IsFileExist(l.basePath); exist == false {
		err = os.MkdirAll(l.basePath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	// log file
	if _, exist, size := IsFileExist(l.fileName); exist == true {
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

func (l *Glogger) Output(level string, isLn bool, format string, v ...interface{}) {
	configLevel, ok := logLevelInfo[l.logConf.Level]
	if !ok {
		configLevel = logLevelInfo["All"]
	}
	if inputLevel, _ := logLevelInfo[level]; configLevel > inputLevel {
		return
	}
	var s string
	if isLn {
		s = fmt.Sprint(v...)
	} else {
		s = fmt.Sprintf(format, v...)
	}
	var sLen int = 0
	switch level {
	case "Info":
		sLen, _ = l.log.Println("[Info] ", s)
	case "Warn":
		sLen, _ = l.log.Println("[Warn] ", s)
	case "Debug":
		sLen, _ = l.log.Println("[Debug] ", s)
	case "Error":
		sLen, _ = l.log.Println("[Error] ", s)
	case "Panic":
		sLen, _ = l.log.Panicln("[Panic] ", s)
		panic(s)
	case "Fatal":
		sLen, _ = l.log.Fatalln("[Fatal] ", s)
		os.Exit(1)
	}
	// mode  debug   return
	if len(l.mode) > 0 && l.mode == "debug" {
		return
	}
	l.rollingLock.Lock()
	defer l.rollingLock.Unlock()
	l.currentSize += int64(sLen)
	// fmt.Println(" l.currentSize  %d   maxSize   %d", l.currentSize, l.logConf.MaxLogSize)
	if l.currentSize >= l.logConf.MaxLogSize {
		l.RollingFile()
	}

}

func (l *Glogger) Done() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

func (l *Glogger) RollingFile() {

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

func (l *Glogger) Infof(format string, v ...interface{}) {
	l.Output("Info", false, format, v...)
}

func (l *Glogger) Infoln(v ...interface{}) {
	l.Output("Info", true, "", v...)
}

func (l *Glogger) Warnf(format string, v ...interface{}) {
	l.Output("Warn", false, format, v...)
}
func (l *Glogger) Warnln(v ...interface{}) {
	l.Output("Warn", true, "", v...)
}

func (l *Glogger) Debugf(format string, v ...interface{}) {
	l.Output("Debug", false, format, v...)
}

func (l *Glogger) Debugln(v ...interface{}) {
	l.Output("Debug", true, "", v...)
}

func (l *Glogger) Errorf(format string, v ...interface{}) {
	l.Output("Error", false, format, v...)
}

func (l *Glogger) Errorln(v ...interface{}) {
	l.Output("Error", true, "", v...)
}

func (l *Glogger) Panicf(format string, v ...interface{}) {
	l.Output("Panic", false, format, v...)
}

func (l *Glogger) Panicln(v ...interface{}) {
	l.Output("Panic", true, "", v...)
}

func (l *Glogger) fatalf(format string, v ...interface{}) {
	l.Output("Fatal", false, format, v...)
}
func (l *Glogger) fatalln(v ...interface{}) {
	l.Output("Fatal", true, "", v...)
}
