// Copyright 2024 lke-operator contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-resty/resty/v2"
)

type Logger struct {
	loggerImpl logr.Logger
}

var _ resty.Logger = (*Logger)(nil)

var ErrOnResty = errors.New("error from resty")

func Wrap(logger logr.Logger) *Logger {
	return &Logger{
		loggerImpl: logger,
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.loggerImpl.Error(ErrOnResty, fmt.Sprintf(format, v...))
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.loggerImpl.Info(fmt.Sprintf(format, v...))
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	l.loggerImpl.V(8).Info(fmt.Sprintf(format, v...))
}
