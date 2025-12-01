// 這個檔案是為了讓你可以直接在專案根目錄執行
// 實際的程式碼在 backend/main.go
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func main() {
	fmt.Println("Starting Go-Travel-Planner...")
	fmt.Println("================================")

	// 切換到 backend 目錄並執行
	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = "./backend"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
