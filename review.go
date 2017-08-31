package main

import (
    "fmt"
    "net/http"
    "sync"
    "github.com/satori/go.uuid"
    "io/ioutil"
)

var sessionStore map[string]Client
var storageMutex sync.RWMutex
var managers map[string]string

type Client struct {
    loggedIn bool
    username string
}

const loginPage = `<html>
<head>
    <title>Login</title>
</head>
<body>
    <form id="login" action="foo" method="post">
        <input type="input" name="user" />
        <input type="password" name="password" />
        <input type="submit" value="Login" />
    </form>
    <script>
    function SetLoginUrl() {
	var path = window.location.pathname
	if (path.substring(0,"/login".length) == "/login") {
	    path = path.substring("/login".length)
	}
        return "/login"
    }

    document.getElementById('login').setAttribute('action', SetLoginUrl());
    </script>
</body>
</html>`

func main() {

    managers = make(map[string]string)
    sessionStore = make(map[string]Client)
    http.Handle("/scripts/", authenticate(pageHandler{""}))
    http.Handle("/review2017", authenticate(pageHandler{"review.html"}))
    http.HandleFunc("/login", handleLogin)
    http.HandleFunc("/ajax/save", save)
    http.HandleFunc("/ajax/get", getData)

    // read password file
    managers["mcharleb"] = "123"
    fmt.Printf("mcharleb "+managers["mcharleb"])

    http.ListenAndServe(":8000", nil)
}

type pageHandler struct {
    redir string
}

func (h pageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if h.redir != "" {
	fmt.Printf("Serving redir %s\n", h.redir)
        http.ServeFile(w, r, h.redir)
    } else {
	fmt.Printf("Serving %s\n", r.URL.Path[1:])
        http.ServeFile(w, r, r.URL.Path[1:])
    }
}

func checkCookie(w http.ResponseWriter, r *http.Request) bool {
    cookie, err := r.Cookie("session")
    if err != nil {
        if err != http.ErrNoCookie {
            fmt.Fprint(w, err)
            return false
        } else {
            err = nil
        }
    }
    fmt.Printf("Cookie %s\n", cookie.Value)
    var present bool
    var client Client
    storageMutex.RLock()
    client, present = sessionStore[cookie.Value]
    storageMutex.RUnlock()

    if present == false {
        fmt.Printf("Error cookie not found\n")
        return false
    }
    if !client.loggedIn == true {
        fmt.Printf("User not logged in\n")
        return false
    }
    fmt.Printf("got client user %s\n", client.username)
    http.SetCookie(w, cookie)
    return true
}

type authenticationMiddleware struct {
    wrappedHandler http.Handler
}

func (h authenticationMiddleware) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    fmt.Printf("authenticationMiddleware\n")
    if !checkCookie(w, r) {
        fmt.Printf("Error: cookie check failed\n")
        fmt.Printf("show login page\n")
        fmt.Fprint(w, loginPage)
        return
    } else {
        fmt.Printf("show page "+r.URL.Path[:]+"\n")
        h.wrappedHandler.ServeHTTP(w, r)
        return
    }
}

func authenticate(h http.Handler) authenticationMiddleware {
    return authenticationMiddleware{h}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("handleLogin\n")
    cookie, err := r.Cookie("session")
    if err != nil {
        if err != http.ErrNoCookie {
            fmt.Fprint(w, err)
            return
        } else {
            err = nil
        }
    }
    var present bool
    var client Client
    if cookie != nil {
        storageMutex.RLock()
        client, present = sessionStore[cookie.Value]
        storageMutex.RUnlock()
    } else {
        present = false
    }

    if present == false {
        cookie = &http.Cookie{
            Name: "session",
            Value: uuid.NewV4().String(),
        }
        client = Client{false, ""}
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
        fmt.Printf("Cookie set %s\n", cookie.Value)
    }
    fmt.Printf("parsing form\n")
    http.SetCookie(w, cookie)
    err = r.ParseForm()
    if err != nil {
        fmt.Fprint(w, err)
        return
    }

    fmt.Printf("Login "+r.FormValue("user")+" "+r.FormValue("password")+"\n")
    if (managers[r.FormValue("user")] == r.FormValue("password")) {
        fmt.Printf("Login good - loading \n")
        //login user
        client.loggedIn = true
        client.username = r.FormValue("user")
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
        fmt.Printf("USER:"+client.username+"\n")
        http.Redirect(w, r, "/review2017", http.StatusFound)
    } else {
        fmt.Fprint(w, loginPage)
        fmt.Fprintln(w, "Wrong password.")
    }
}

func save(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("save\n")
    fmt.Printf("Parsing form\n")
    var user = checkCookie(w, r)
    if user == "" {
        fmt.Printf("Error: cookie check failed\n")
        return
    }
    err := r.ParseForm()
    if err != nil {
        fmt.Fprint(w, err)
        return
    }

    var formuser = r.FormValue("user")
    var data = r.FormValue("data")
    if user != formuser {
        fmt.Printf("Error: user not provided\n")
    } else {
        ioutil.WriteFile("data/"+user+".js", []byte(data), 0644)
    }
    w.Write([]byte("{}"))
}

func getData(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("getData\n")
    var user = checkCookie(w, r)
    if user == "" {
        fmt.Printf("Error: cookie check failed\n")
        return
    }

    // read saved json data
    tabledata, err := ioutil.ReadFile(user+".js")
    fmt.Printf("tabledata 1\n")
    if err != nil {
        // Load original data
        tabledata, err = ioutil.ReadFile("data/unranked.js")
        fmt.Printf("tabledata 2\n")
        if err != nil {
            fmt.Fprint(w, err)
            fmt.Printf("tabledata failed\n")
            return
        }
    }
    fmt.Printf("tabledata 3 %s\n", tabledata)
    w.Write([]byte("{\"user\":\""+user+"\", \"tabledata\":"))
    w.Write(tabledata)
    w.Write([]byte("}"))
}
