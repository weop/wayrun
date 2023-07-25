package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type App struct {
	Name string `json:"name"`
	Exec string `json:"exec"`
}

func main() {
	application := app.New()
	window := application.NewWindow("wayrun")

	var apps []App
	var btnSelected *widget.Button

	window.Resize(fyne.Size{Width: 500, Height: 280})
	window.SetFixedSize(true)
	window.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			application.Quit()
		}
	})

	// add flatpaks from /.local/share/flatpak/exports/bin
	flatpakPath := os.Getenv("HOME") + "/.local/share/flatpak/exports/bin"
	// log.Println(flatpakPath + " is flatpak path")
	files, err := ioutil.ReadDir(flatpakPath)
	if err != nil {
		log.Println(err)
	} else {
		for _, file := range files {
			if !file.IsDir() {

				appExec := "flatpak run " + file.Name()

				//lowercase the app name
				appName := strings.ToLower(file.Name())

				app := App{
					Name: appName,
					Exec: appExec,
				}

				apps = append(apps, app)

			}
		}
	}

	//add apps from desktop files
	dirs := []string{
		"/usr/local/share/applications",
		"~/.local/share/applications",
		"/usr/share/applications",
	}

	loadAppsFromDirectories(dirs)

	//add applications from env paths
	pathEnv := os.Getenv("PATH")
	paths := filepath.SplitList(pathEnv)

	for _, path := range paths {
		//if path is in a sbin, skip it
		if strings.Contains(path, "sbin") {
			continue
		}

		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Println(err)
			continue
		}

		for _, file := range files {
			if !file.IsDir() {
				appName := file.Name()

				app := App{
					Name: appName,
					Exec: "bash -c \"" + appName + "\" > /dev/null",
				}

				apps = append(apps, app)
			}
		}
	}

	apps = cleanAppList(apps)

	// jsonApps, err := json.MarshalIndent(apps, "", "  ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// //fmt.Println(string(jsonApps))

	filterEntry := widget.NewEntry()
	buttonBox := container.NewVBox()

	filterEntry.OnChanged = func(uinput string) {

		//remove all digits from uinput
		query := strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return -1
			}
			return r
		}, uinput)

		//trigger to quit
		if strings.HasSuffix(query, "qq") {
			application.Quit()
		}

		buttons := makeAppButtons(apps, query, application)
		buttonBox.Objects = buttons
		buttonBox.Refresh()

		if len(buttons) > 0 {
			btnSelected = buttons[0].(*widget.Button)
		} else {
			btnSelected = nil
		}

		digitStr := ""
		for i := len(uinput) - 1; i >= 0; i-- {
			if unicode.IsDigit(rune(uinput[i])) {
				digitStr = string(uinput[i])
				break
			}
		}

		if digitStr != "" {
			digit, err := strconv.Atoi(digitStr)
			if err == nil && 1 <= digit && digit <= 5 {
				log.Printf("Triggering button %d\n", digit)
				if len(buttons) > 0 {
					btnSelected = buttons[digit-1].(*widget.Button)
				} else {
					log.Printf("No buttons to trigger\n")
					btnSelected = nil
				}
				if btnSelected == nil {
					return
				}

				btnSelected.Tapped(nil)
			}
		}

		if digit, err := strconv.Atoi(uinput); err == nil && digit > 0 && digit <= len(buttons) {
			button := buttons[digit-1].(*widget.Button)
			button.Tapped(nil)
			application.Quit()
		}
	}

	filterEntry.OnSubmitted = func(text string) {
		if btnSelected != nil {
			btnSelected.Tapped(nil)
		} else {
			application.Quit()
		}
	}

	box := container.NewVBox(filterEntry, buttonBox)

	window.SetContent(box)
	window.Canvas().Focus(filterEntry)
	window.CenterOnScreen()
	window.ShowAndRun()
}

func loadAppsFromDirectories(dirs []string) []App {
	var apps []App

	for _, dir := range dirs {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".desktop") {
				continue
			}

			execLine, nameLine, err := getExecLine(dir + "/" + file.Name())
			if err != nil {
				continue
			}

			nameLine = strings.TrimSuffix(file.Name(), ".desktop") + " - " + nameLine
			nameLine = strings.Title(nameLine)

			app := App{
				Name: nameLine,
				Exec: execLine,
			}
			apps = append(apps, app)
		}
	}

	return apps
}

func makeAppButtons(apps []App, filter string, application fyne.App) []fyne.CanvasObject {
	var buttons []fyne.CanvasObject
	var btnLength int = 1

	for _, app := range apps {
		if filter != "" && !strings.Contains(app.Name, filter) {
			continue
		}
		buttonName := strconv.Itoa(btnLength) + " " + app.Name
		button := widget.NewButton(buttonName, func(app App) func() {
			return func() {
				parts := strings.Split(app.Exec, " ")
				cmd := exec.Command(parts[0], parts[1:]...)
				err := cmd.Start()
				application.Quit()

				if err != nil {
					log.Printf("cmd.Run() failed with %s\n", err)
					//application.Quit()
				}
				application.Quit()
			}
		}(app))

		buttons = append(buttons, button)
		btnLength = len(buttons) + 1

		// Stop creating buttons once we have 5
		if btnLength >= 6 {
			break
		}

	}

	return buttons
}

func getExecLine(desktopFilePath string) (string, string, error) {
	file, err := os.Open(desktopFilePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var execLine string
	var nameLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Exec=") {
			launcherfile := strings.Split(desktopFilePath, "/")
			launcherfile = launcherfile[len(launcherfile)-1:]
			desktopFilePath = strings.Join(launcherfile, "/")
			execLine = "gtk-launch " + desktopFilePath + " & 2> /dev/null"
			log.Printf("Exec line: %s\n", execLine)
			//execLine2 = strings.TrimPrefix(line, "Exec=")
		} else if strings.HasPrefix(line, "Name=") {
			nameLine = strings.TrimPrefix(line, "Name=")
		}
	}

	if execLine == "" || nameLine == "" {
		return "", "", fmt.Errorf("Exec or Name line not found")
	}

	return execLine, nameLine, nil
}

func cleanAppList(apps []App) []App {
	seen := make(map[string]bool)
	result := []App{}

	for _, app := range apps {

		//some filtering
		//if app name contains "shell" or "gnome" then skip
		if strings.Contains(app.Name, "-shell") || strings.Contains(app.Name, "-gnome") {
			continue
		}
		if _, ok := seen[app.Name]; !ok {
			seen[app.Name] = true
			result = append(result, app)
		}
	}

	return result
}
