package cli

import (
	"flag"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWithPositionals(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	paths := fs.String("paths", "", "")
	intro := fs.String("intro", "", "")
	verbose := fs.Bool("verbose", false, "")
	args := []string{"/some/dir", "--paths", "wiki,issues", "-verbose", "--intro=一句话"}
	if err := ParseWithPositionals(fs, args); err != nil {
		t.Fatal(err)
	}
	if *paths != "wiki,issues" {
		t.Errorf("paths = %q", *paths)
	}
	if *intro != "一句话" {
		t.Errorf("intro = %q", *intro)
	}
	if !*verbose {
		t.Error("bool 旗标应生效")
	}
	if fs.NArg() != 1 || fs.Arg(0) != "/some/dir" {
		t.Errorf("位置参数 = %v", fs.Args())
	}
}

func TestSplitCSV(t *testing.T) {
	got := SplitCSV(" 技术笔记, AI ,,笔记,")
	want := "技术笔记|AI|笔记"
	if strings.Join(got, "|") != want {
		t.Errorf("SplitCSV = %v, want %s", got, want)
	}
}

func TestUnderOrEqualAndSamePath(t *testing.T) {
	if !UnderOrEqual(filepath.Join("a", "b"), "a") {
		t.Error("子目录应判定在其下")
	}
	if UnderOrEqual("abc", "a") {
		t.Error("前缀相同但非子目录不应误判")
	}
	if !SamePath(filepath.Join("A", "B"), filepath.Join("a", "b")) {
		t.Error("大小写不敏感比较失败")
	}
}

func TestParseBool(t *testing.T) {
	yes := []string{"true", "TRUE", "1", "yes", "On"}
	for _, s := range yes {
		b, err := ParseBool(s)
		if err != nil || !b {
			t.Errorf("ParseBool(%q) = %v, %v", s, b, err)
		}
	}
	no := []string{"false", "0", "no", "OFF"}
	for _, s := range no {
		b, err := ParseBool(s)
		if err != nil || b {
			t.Errorf("ParseBool(%q) = %v, %v", s, b, err)
		}
	}
	if _, err := ParseBool("maybe"); err == nil {
		t.Error("非法值应报错")
	}
}
