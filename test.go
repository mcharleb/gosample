package main

import (
    "fmt"
    "net/http"
    "sync"
    "github.com/satori/go.uuid"
    "crypto/subtle"
)

var sessionStore map[string]Client
var storageMutex sync.RWMutex

type Client struct {
    loggedIn bool
}

const loginPage = `<html>
<head>
    <title>Login</title>
</head>
<body>
    <form id="login" action="foo" method="post"> <input type="password" name="password" />
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

    sessionStore = make(map[string]Client)
    http.Handle("/", authenticate(helloWorldHandler{}))
    http.HandleFunc("/login", handleLogin)

    http.ListenAndServe(":8000", nil)
}

type helloWorldHandler struct {
}

func (h helloWorldHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    http.ServeFile(w, r, r.URL.Path[1:])
}

type authenticationMiddleware struct {
    wrappedHandler http.Handler
}

func (h authenticationMiddleware) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    fmt.Printf("authenticationMiddleware\n")
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
        client = Client{false}
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
    }
    http.SetCookie(w, cookie)
    if client.loggedIn == false {
        fmt.Printf("show login page\n")
        fmt.Fprint(w, loginPage)
        return
    }
    if client.loggedIn == true {
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
        client = Client{false}
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
    }
    fmt.Printf("parsing form\n")
    http.SetCookie(w, cookie)
    err = r.ParseForm()
    if err != nil {
        fmt.Fprint(w, err)
        return
    }

    fmt.Printf("Test Login "+r.FormValue("password")+" xx\n")
    if subtle.ConstantTimeCompare([]byte(r.FormValue("password")), []byte("123")) == 1 {
        fmt.Printf("Login good - loading \n")
        //login user
        client.loggedIn = true
	http.ServeFile(w, r, "managees.html")
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
    } else {
        fmt.Fprint(w, loginPage)
        fmt.Fprintln(w, "Wrong password.")
    }
}
