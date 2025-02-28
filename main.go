package main

import (
    "database/sql"
    "fmt"
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
    ID    int
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
    todoList := tview.NewTextView()
    todoList.SetBorder(false)
    todoList.SetBackgroundColor(tcell.ColorDefault)

    refreshTodos := func() {
        todoList.Clear()
        todos, err := getTodos()
        if err != nil {
            return
        }
        if len(todos) == 0 {
            fmt.Fprintln(todoList, "Nothing to do!")
        } else {
            for _, todo := range todos {
                fmt.Fprintf(todoList, "%s\n", todo.Title)
            }
        }
    }
    refreshTodos()

    if err := app.SetRoot(todoList, true).Run(); err != nil {
        log.Fatal(err)
    }
}
