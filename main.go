package main

import (
    "database/sql"
    "log"
    "os"
    "path"
    "path/filepath"

    "github.com/gdamore/tcell/v2"
    "github.com/rivo/tview"
    "golang.org/x/sys/unix"
    _ "modernc.org/sqlite"
)

type Todo struct {
    ID    int64
    Title string
}

var DB *sql.DB

func writable(path string) bool {
    return unix.Access(path, unix.W_OK) == nil
}

func getDBPath() (string, error) {
    dbFileName := "db.sqlite"

    // Prefer XDG_DATA_HOME
    xdgDataHome, exists := os.LookupEnv("XDG_DATA_HOME")
    if exists && writable(xdgDataHome) {
        path := filepath.Join(xdgDataHome, "gdn")
        os.MkdirAll(path, os.ModePerm)
        return filepath.Join(path, dbFileName), nil
    }

    // fallback to $HOME/.local/share
    homeDir, exists := os.LookupEnv("HOME")
    localShare := path.Join(homeDir, ".local", "share")
    if exists && writable(localShare) {
        path := filepath.Join(localShare, "gdn")
        os.MkdirAll(path, os.ModePerm)
        return filepath.Join(path, dbFileName), nil
    }

    return "", os.ErrNotExist
}

func initDB() error {
    dbPath, err := getDBPath()
    DB, err = sql.Open("sqlite", dbPath)
    if err != nil {
        return err
    }

    _, err = DB.Exec(
        `CREATE TABLE IF NOT EXISTS todos (
        id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
        title TEXT
        );`,
        )
    if err != nil {
        return err
    }
    return nil
}

func createTodo(title string) (int64, error) {
    res, err := DB.Exec("INSERT INTO todos(title) VALUES(?);", title)
    if err != nil {
        return -1, err
    }
    id, err := res.LastInsertId()
    if err != nil {
        return -1, err
    }
    return id, nil
}

func deleteTodo(id int64) error {
    _, err := DB.Exec("DELETE FROM todos WHERE id = ?;", id)
    return err
}

func getTodos() ([]Todo, error) {
    rows, err := DB.Query("SELECT id, title FROM todos;")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    todos := []Todo{}
    for rows.Next() {
        var todo Todo
        if err := rows.Scan(&todo.ID, &todo.Title); err != nil {
            return nil, err
        }
        todos = append(todos, todo)
    }
    return todos, nil
}

func main() {
    if err := initDB(); err != nil {
        log.Fatal(err)
    }
    defer DB.Close()

    app := tview.NewApplication()
    pages := tview.NewPages()

    var todos []Todo

    todoList := tview.NewList()
    todoList.
        SetBorder(false).
        SetBackgroundColor(tcell.ColorDefault)

    refreshTodos := func() {
        todoList.Clear()
        dbTodos, err := getTodos()
        if err != nil {
            return
        }
        if len(dbTodos) == 0 {
            todoList.AddItem("Nothing to do!", "", rune(0), nil).
                SetBackgroundColor(tcell.ColorDefault)
        } else {
            for _, todo := range dbTodos {
                todoList.AddItem(todo.Title, "", rune(0), nil)
            }
        }
        todos = dbTodos
    }

    textInput := tview.NewInputField()
    textInput.SetDoneFunc(func(key tcell.Key) {
        if key == tcell.KeyEnter {
            createTodo(textInput.GetText())
            refreshTodos()
        }
        pages.RemovePage("add item modal")
    })

    textInputPopup := tview.NewFlex().
        SetDirection(tview.FlexRow).
        AddItem(textInput, 0, 1, true)
    textInputPopup.SetBorder(true).SetTitle("New Todo")

    modal := tview.NewFlex().
        AddItem(nil, 0, 1, false).
        AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
            AddItem(nil, 0, 1, false).
            AddItem(textInputPopup, 3, 1, true).
            AddItem(nil, 0, 1, false), 50, 1, true).
        AddItem(nil, 0, 1, false)
    todoList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
        switch event.Rune() {
        case 'j':
            if todoList.GetCurrentItem() < todoList.GetItemCount() {
                todoList.SetCurrentItem(todoList.GetCurrentItem() + 1)
            }
        case 'k':
            if todoList.GetCurrentItem() > 0 {
                todoList.SetCurrentItem(todoList.GetCurrentItem() - 1)
            }
        case 'o':
            textInput.SetText("")
            pages.AddPage("add item modal", modal, true, true)
        case 'd':
            i := todoList.GetCurrentItem()
            deleteTodo(todos[i].ID)
            refreshTodos()
            if i >= len(todos) {
                i = max(0, i-1)
            }
            todoList.SetCurrentItem(i)
        }
        return event
    })

    pages.AddPage("main", todoList, true, true)
    pages.
        SetBorder(false).
        SetBackgroundColor(tcell.ColorDefault)

    refreshTodos()
    if err := app.SetRoot(pages, true).Run(); err != nil {
        log.Fatal(err)
    }
}
