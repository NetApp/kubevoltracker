/*
   Copyright 2016 Chris Dragga <cdragga@netapp.com>

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/netapp/kubevoltracker/dbmanager/mysql"
	"github.com/netapp/kubevoltracker/resources"
)

var (
	mySQLUser     string
	mySQLPassword string
)

func init() {
	const (
		defaultUser     = "root"
		defaultPassword = "root"
		userUsage       = "MySQL username"
		passwordUsage   = "MySQL password"
	)

	flag.StringVar(&mySQLUser, "username", defaultUser, userUsage)
	flag.StringVar(&mySQLUser, "u", defaultUser, userUsage+" (shorthand)")
	flag.StringVar(&mySQLPassword, "password", defaultPassword, passwordUsage)
	flag.StringVar(&mySQLPassword, "p", defaultPassword, passwordUsage+
		" (shorthand)")
	if os.Getenv("MYSQL_IP") == "" {
		log.Fatal("ERROR: Must specify IP address of MYSQL server in MYSQL_IP.")
	}
	if os.Getenv("KUBERNETES_MASTER") == "" {
		log.Fatal("ERROR: Must specify IP address of Kubernetes master in " +
			"KUBERNETES_MASTER.")
	}
}

func main() {
	flag.Parse()
	manager := mysql.New(mySQLUser, mySQLPassword, os.Getenv("MYSQL_IP"))
	w, err := NewWatcher("", os.Getenv("KUBERNETES_MASTER"), manager)
	if err != nil {
		log.Fatal("Unable to create watcher: ", err)
	}
	defer w.Destroy()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	w.Watch(resources.Pods, false)
	w.Watch(resources.PVs, false)
	w.Watch(resources.PVCs, false)

	<-c
	log.Print("Shutting down")
}
