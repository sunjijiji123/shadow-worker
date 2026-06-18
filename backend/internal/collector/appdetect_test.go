package collector

import (
	"fmt"
	"testing"
)

func TestForegroundApp(t *testing.T) {
	app, err := ForegroundApp()
	if err != nil {
		t.Fatalf("ForegroundApp 失败: %v", err)
	}
	if app.Path == "" {
		t.Fatal("前台应用路径不应为空")
	}
	if app.Name == "" {
		t.Fatal("前台应用名不应为空")
	}
	fmt.Printf("ForegroundApp: path=%s name=%s title=%s pid=%d\n",
		app.Path, app.Name, app.WindowTitle, app.PID)
}
