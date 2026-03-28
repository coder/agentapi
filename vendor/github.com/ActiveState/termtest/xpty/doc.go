// Copyright 2020 ActiveState Software. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file

// Package xpty is an abstraction of a pseudoterminal that is attached to a
// virtual xterm-compatible terminal.
//
// This can be used to automate the execution of terminal applications that rely
// on running inside of a real terminal: Especially if the terminal application
// sends a cursor position request (CPR) signal, it usually blocks on read until
// it receives the response (the column and row number of the cursor) from
// terminal.
//
// The state of the virtual terminal can also be accessed at any point. So, the
// output displayed to a user running the application in a "real" terminal can
// be inspected and analyzed.
package xpty
