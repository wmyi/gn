package gnError

import (
	"context"
	"gn/glog"
	"runtime/debug"
)

type ExceptionHandleFunc func(exception *GnException)

type GnExceptionDetect struct {
	isRuning          bool
	exceptionChan     chan *GnException
	detectRoutineCanl context.CancelFunc
	logger            *glog.Glogger
	exceptionHandlers []ExceptionHandleFunc
}

func NewGnExceptionDetect(log *glog.Glogger) *GnExceptionDetect {
	return &GnExceptionDetect{
		isRuning:          false,
		exceptionChan:     make(chan *GnException, 1<<9),
		logger:            log,
		exceptionHandlers: make([]ExceptionHandleFunc, 1<<5),
	}
}

func (ge *GnExceptionDetect) Run(isStartRoutine bool) error {
	if !ge.isRuning {
		ge.isRuning = true
		if isStartRoutine {
			ctx, cancal := context.WithCancel(context.Background())
			go ge.loopDetectException(ctx)
			ge.detectRoutineCanl = cancal
		} else {
			ge.loopDetectException(nil)
		}
		return nil
	} else {
		return ErrExceptionDetectRuning
	}

}
func (ge *GnExceptionDetect) AddExceptionHandler(handler ExceptionHandleFunc) {
	if handler != nil {
		ge.exceptionHandlers = append(ge.exceptionHandlers, handler)
	}
}

func (ge *GnExceptionDetect) ThrowException(exception *GnException) {
	if exception != nil {
		ge.exceptionChan <- exception
	}
}

func (ge *GnExceptionDetect) loopDetectException(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			ge.logger.Errorf("GnExceptionDetect loopRoutine ", string(debug.Stack()))
		}
	}()
	for {
		if ctx == nil {
			if data, ok := <-ge.exceptionChan; ok && data != nil {
				ge.callBackhandler(data)
			} else {
				ge.logger.Errorf("expceptionChan  channel <- error  ")
				panic(ErrExceptionDetectChan)
			}
		} else {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-ge.exceptionChan:
				if ok && data != nil {
					ge.callBackhandler(data)
				} else {
					ge.logger.Errorf("expceptionChan  channel <- error  ")
					panic(ErrExceptionDetectChan)
				}
			}
		}

	}
}

func (ge *GnExceptionDetect) callBackhandler(ex *GnException) {
	if len(ge.exceptionHandlers) > 0 {
		for _, handler := range ge.exceptionHandlers {
			if handler != nil {
				handler(ex)
			}
		}
	}
}

func (ge *GnExceptionDetect) Done() {
	if ge.isRuning {
		if ge.detectRoutineCanl != nil {
			ge.detectRoutineCanl()
		}
		if len(ge.exceptionHandlers) > 0 {
			ge.exceptionHandlers = nil
		}
		ge.isRuning = false

	}
}
