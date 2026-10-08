package main

import (
	"strings"
	"testing"
)

// testDump is a reduced reference dump: an easy Wireshark question with two
// sub-questions listed out of order, a hard Wireshark question, and an Nmap
// question, written as mysqldump writes them.
const testDump = "-- MySQL dump\n" +
	"INSERT INTO `questions` VALUES " +
	"(1,'wireshark','question 1','Messages from 4444 to 9999 don\\'t agree.\\nFind the true one.','PCAP/question1.pcap','','easy',50,'2026-03-20 12:07:07')," +
	"(2,'wireshark','question 2','One flag.','PCAP/scenario6.pcap','','hard',50,'2026-03-20 12:07:07')," +
	"(3,'nmap','question 3','Scan the target.','','Scenario1',' Hard ',50,'2026-03-20 12:07:07');\n" +
	"INSERT INTO `questions_flags` VALUES (1,1,'D935e3',30),(2,2,'BITSATHY',50),(3,3,'f7f4d1',50);\n" +
	"INSERT INTO `subquestions` VALUES (1,1,'Second sub-question',2),(2,1,'First sub-question',1);\n" +
	"INSERT INTO `subquestions_flags` VALUES (1,1,'0xc9ab',8),(2,2,'41.133',12);\n" +
	"INSERT INTO `users` VALUES (1,'7376222AG102','someone@example.edu','Someone',NULL);\n"

func readTestDump(t *testing.T, dump string) map[string][][]value {
	t.Helper()
	tables, err := readDump(strings.NewReader(dump))
	if err != nil {
		t.Fatalf("readDump() error = %v", err)
	}
	return tables
}

func TestReadDumpUnescapesValues(t *testing.T) {
	tables := readTestDump(t, testDump)

	if got := len(tables["questions"]); got != 3 {
		t.Fatalf("questions rows = %d; want 3", got)
	}
	if got := tables["questions"][0][3].text; got != "Messages from 4444 to 9999 don't agree.\nFind the true one." {
		t.Fatalf("description = %q; want escapes undone", got)
	}
	user := tables["users"][0]
	if !user[4].null || user[3].null || user[3].text != "Someone" {
		t.Fatalf("users row = %+v; want NULL distinguished from text", user)
	}
}

func TestReadDumpRejectsMalformedRows(t *testing.T) {
	for _, dump := range []string{
		"INSERT INTO `t` VALUES (1,'open);\n",
		"INSERT INTO `t` VALUES (1,2)\n",
		"INSERT INTO `t` VALUES (1,,2);\n",
		"INSERT INTO `t` VALUES (1,2);x\n",
	} {
		if _, err := readDump(strings.NewReader(dump)); err == nil {
			t.Errorf("readDump(%q) error = nil; want a parse error", dump)
		}
	}
}

func TestBuildBank(t *testing.T) {
	bank, err := buildBank(readTestDump(t, testDump))
	if err != nil {
		t.Fatalf("buildBank() error = %v", err)
	}
	if len(bank) != 3 {
		t.Fatalf("bank has %d questions; want 3", len(bank))
	}

	easy := bank[0]
	if easy.pool != "wireshark" || easy.difficulty != "easy" || easy.pcapFile != "question1.pcap" {
		t.Fatalf("easy question = %+v", easy)
	}
	wantPrompts := []string{"Submit the flag.", "First sub-question", "Second sub-question"}
	wantAnswers := []string{"D935e3", "41.133", "0xc9ab"}
	if len(easy.flags) != 3 {
		t.Fatalf("easy flags = %+v; want main flag then sub-questions in order", easy.flags)
	}
	for i, f := range easy.flags {
		if f.prompt != wantPrompts[i] || f.answer != wantAnswers[i] {
			t.Errorf("flag %d = %+v; want %q / %q", i, f, wantPrompts[i], wantAnswers[i])
		}
	}

	nmap := bank[2]
	if nmap.pool != "nmap" || nmap.image != "scenario1" || nmap.difficulty != "hard" || nmap.pcapFile != "" {
		t.Fatalf("nmap question = %+v; want lower-case image and difficulty", nmap)
	}
}

func TestBuildBankRejectsIncompleteQuestions(t *testing.T) {
	tests := map[string]struct{ old, new, wantErr string }{
		"wrong total":       {"(3,3,'f7f4d1',50)", "(3,3,'f7f4d1',40)", "total 40 points"},
		"missing main flag": {"(3,3,'f7f4d1',50)", "(3,99,'f7f4d1',50)", "0 main flags"},
		"no image":          {"'Scenario1'", "''", "no docker image"},
		"bad difficulty":    {"' Hard '", "'medium'", "difficulty"},
		"empty answer":      {"'BITSATHY'", "' '", "empty answer"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dump := strings.Replace(testDump, tt.old, tt.new, 1)
			_, err := buildBank(readTestDump(t, dump))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("buildBank() error = %v; want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
