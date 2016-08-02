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
	"testing"
)

// This is dumb, but my latent OCD is getting the better of me.
// TODO:  Smart stuff with INTMAX/INTMIN.
func TestMax(t *testing.T) {
	if m := max(2, 10); m != 10 {
		t.Error("Expected 10; got ", m)
	}
	if m := max(10, 2); m != 10 {
		t.Error("Expected 10; got ", m)
	}
	if m := max(-5, -3); m != -3 {
		t.Error("Expected -3; got ", m)
	}
	if m := max(-100000000, 100000000); m != 100000000 {
		t.Error("Expected 100000000; got ", m)
	}
	if m := max(0, -0); m != 0 {
		t.Error("Expected 0; got ", m)
	}
}

func TestGetCommandString(t *testing.T) {
	if s := GetCommandString([]string{"/bin/sh"}); s != "/bin/sh" {
		t.Error("Incorrect string value; expected /bin/sh, got ", s)
	}
	if s := GetCommandString([]string{"sleep", "120"}); s != "sleep 120" {
		t.Errorf("Incorrect string value; expected \"sleep 120\", got \"%s\"\n",
			s)
	}
}
