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

    managers = make(map[string]string)
    sessionStore = make(map[string]Client)
    http.Handle("/scripts/", authenticate(pageHandler{""}))
    http.Handle("/review2017", authenticate(pageHandler{"review.html"}))
    http.HandleFunc("/login", handleLogin)
    http.HandleFunc("/ajax/save", save)
    http.HandleFunc("/ajax/get", getData)

    // read password file
    managers["mcharleb"] = "123"
    managers["rkumar"] = "123"

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

func checkCookie(w http.ResponseWriter, r *http.Request) string {
    fmt.Printf("CHECK COOKIE\n")
    var client Client
    var present bool = false
    for _, cookie := range r.Cookies() {
        fmt.Printf("%s %s\n",cookie.Name, cookie.Value)
	if cookie.Name == "session" {
	    storageMutex.RLock()
	    client, present = sessionStore[cookie.Value]
	    storageMutex.RUnlock()

            if present == true {
                http.SetCookie(w, cookie)
	        break
            }
        }
    }
    if present == false {
        fmt.Printf("Error cookie not found\n")
        return ""
    }
    if !client.loggedIn == true {
        fmt.Printf("User not logged in\n")
        return ""
    }
    fmt.Printf("got client user %s\n", client.username)
    return client.username
}

type authenticationMiddleware struct {
    wrappedHandler http.Handler
}

func (h authenticationMiddleware) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    fmt.Printf("authenticationMiddleware\n")
    if checkCookie(w, r) == "" {
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
    tabledata, err := ioutil.ReadFile("data/"+user+".js")
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
