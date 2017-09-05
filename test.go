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
<style>
.container {
  width:210px;
  margin:0 auto;
    padding: 16px;
}
input[type=input], input[type=password] {
    width: 100%;
    padding: 12px 20px;
    margin: 8px 0;
    display: inline-block;
    border: 1px solid #ccc;
    box-sizing: border-box;
}
button {
    background-color: #4CAF50;
    color: white;
    padding: 14px 20px;
    margin: 8px 0;
    border: none;
    cursor: pointer;
    width: 100%;
}
</style>
<body>
    <form id="login" action="/login" method="post">
        <div class="container">
	<label><b>Username</b></label>
	<input type="input" name="user" />
	<label><b>Password</b></label>
	<input type="password" name="password" />
        <button type="submit" />Login</button>
        </div>
    </form>
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
