package main

import (
    "os"
    "fmt"
    "strings"
    "path/filepath"
    "encoding/json"
    "net/http"
    "sync"
    "io/ioutil"
    "github.com/satori/go.uuid"
    "github.com/tealeg/xlsx"
)

var sessionStore map[string]Client
var storageMutex sync.RWMutex

type Client struct {
    loggedIn bool
    username string
}

type Row struct {
    ID  int
    Name string
    YN string
    Rank int
    Notes string
}

type ResultRow struct {
    Name string
    YCount int
    NCount int
    RankTotal int
    YVotes []string
    NVotes []string
}

const loginPage = `<html>
<head>
    <title>Login</title>
<base href="">
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

</style>
<link href="/scripts/buttons.css" rel="stylesheet">
</head>
<body>
    <div class="container">
    <form id="login" action="/login" method="post">
	<label><b>Username</b></label>
	<input type="input" name="user" />
	<label><b>Password</b></label>
	<input type="password" name="pin" />
        <button class="btn" type="submit" />Login</button>
    </form>
    <form id="reset" action="/reset" method="post">
        <button class="btn" type="submit" />Reset PIN</button>
    </form>
    </div>
</body>
</html>`

func main() {

    sessionStore = make(map[string]Client)
    http.Handle("/scripts/", noauthenticate(pageHandler{""}))
    http.Handle("/review2017", authenticate(pageHandler{"review.html"}))
    http.Handle("/results", noauthenticate(pageHandler{"results.html"}))
    http.HandleFunc("/login", handleLogin)
    http.HandleFunc("/reset", handlePinReset)
    http.HandleFunc("/logout", handleLogout)
    http.HandleFunc("/ajax/save", save)
    http.HandleFunc("/ajax/submit", submit)
    http.HandleFunc("/ajax/results", getResults)
    http.HandleFunc("/ajax/get", getData)

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
        path := r.URL.Path
        var contentType string
        if strings.HasSuffix(path, ".css") {
	    fmt.Printf("Serving css\n")
            contentType = "text/css"
        } else {
            contentType = "text/html"
        }
        w.Header().Add("Content-Type", contentType)
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

type noauthenticationMiddleware struct {
    wrappedHandler http.Handler
}

func (h authenticationMiddleware) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    fmt.Printf("authenticationMiddleware"+r.URL.Path[:]+"\n")
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

func (h noauthenticationMiddleware) ServeHTTP(w http.ResponseWriter,r *http.Request) {
    fmt.Printf("noauthenticationMiddleware\n")
    h.wrappedHandler.ServeHTTP(w, r)
}

func authenticate(h http.Handler) authenticationMiddleware {
    return authenticationMiddleware{h}
}

func noauthenticate(h http.Handler) noauthenticationMiddleware {
    return noauthenticationMiddleware{h}
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
    fmt.Printf("Parsing login form\n")
    http.SetCookie(w, cookie)
    err = r.ParseForm()
    if err != nil {
        fmt.Fprint(w, err)
        return
    }

    var user = r.FormValue("user");
    var pin = r.FormValue("pin");
    if (user != "" && pin != "") {
        fmt.Printf("Login "+user+", "+pin+"\n")
        bytepin, err := ioutil.ReadFile(filepath.FromSlash("data/"+user+"/pin.js"))
        if err != nil {
            fmt.Printf("No pin saved\n")
            http.ServeFile(w, r, "reset.html")
            return
        }
        mypin := string(bytepin[:]);
        if (mypin == pin) {
            fmt.Printf("Login good - loading \n")
            //login user
            client.loggedIn = true
            client.username = user;
            storageMutex.Lock()
            sessionStore[cookie.Value] = client
            storageMutex.Unlock()
            fmt.Printf("USER:"+client.username+"\n")
            http.Redirect(w, r, "/review2017", http.StatusFound)
        } else {
            fmt.Printf("Wrong PIN\n")
            fmt.Fprint(w, loginPage)
            fmt.Fprintln(w, "Wrong PIN")
        }
    } else {
        fmt.Printf("Login bad \n")
        fmt.Fprint(w, loginPage)
        fmt.Fprintln(w, "Login or PIN cannot be empty")
    }
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("handleLogout\n")
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

    if present == true {
        client = Client{false, ""}
        client.loggedIn = false;
        storageMutex.Lock()
        sessionStore[cookie.Value] = client
        storageMutex.Unlock()
    } else {
        fmt.Printf("Logout called with no session\n")
    }
    http.Redirect(w, r, "/login", http.StatusFound)
}

func handlePinReset(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("handlePinReset\n")
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
        storageMutex.Lock()
        client, present = sessionStore[cookie.Value]
        if present == true {
            client.loggedIn = false
            sessionStore[cookie.Value] = client
        }
        storageMutex.Unlock()
    } else {
        present = false
    }

    fmt.Printf("Parsing form\n")
    http.SetCookie(w, cookie)
    err = r.ParseForm()
    if err != nil {
        http.ServeFile(w, r, "reset.html")
        return
    }
    fmt.Printf("\n")
    var user = r.FormValue("user");
    var pin = r.FormValue("pin");
    if user == "" || pin == "" {
        http.ServeFile(w, r, "reset.html")
        //fmt.Fprint(w, "Username or PIN cannot be empty")
        return
    }

    fmt.Printf("Reset "+user+" "+pin+"\n")
    _ = os.Mkdir(filepath.FromSlash("data/"+user), os.ModePerm)
    ioutil.WriteFile(filepath.FromSlash("data/"+user+"/pin.js"), []byte(pin), 0644)
    http.Redirect(w, r, "/login", http.StatusFound)
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
        ioutil.WriteFile(filepath.FromSlash("data/"+user+"/save.js"), []byte(data), 0644)
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
    tabledata, err := ioutil.ReadFile(filepath.FromSlash("data/"+user+"/save.js"))
    fmt.Printf("tabledata 1\n")
    if err != nil {
        // Load original data
        tabledata, err = ioutil.ReadFile(filepath.FromSlash("data/promos.js"))
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

func submit(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("submit\n")
    fmt.Printf("Parsing form\n")
    var user = checkCookie(w, r)
    if user == "" {
        fmt.Printf("Error: cookie check failed\n")
        return
    }
    var err error
    err = r.ParseForm()
    if err != nil {
        fmt.Fprint(w, err)
        return
    }

    var formuser = r.FormValue("user")
    if user != formuser {
        fmt.Printf("Error: user not provided\n")
        return
    }
    var data = r.FormValue("data")
    fmt.Println("data:", data)
    // Save data first so this data is reloaded
    ioutil.WriteFile(filepath.FromSlash("data/"+user+"/save.js"), []byte(data), 0644)
    ioutil.WriteFile(filepath.FromSlash("data/"+user+"/submit.js"), []byte(data), 0644)

    var datarows []Row
    err = json.Unmarshal([]byte(data), &datarows)
    if err != nil {
        fmt.Println("error:", err)
    }
    var file *xlsx.File
    var sheet *xlsx.Sheet
    var row *xlsx.Row
    var cell *xlsx.Cell

    file = xlsx.NewFile()
    sheet, err = file.AddSheet("Sheet1")
    if err != nil {
        fmt.Printf(err.Error())
    }
    row = sheet.AddRow()
    cell = row.AddCell()
    cell.Value = user
    for j, v := range datarows {
        fmt.Println("Saving row", j)
        fmt.Println("ID: ", v.ID)
        fmt.Println("Name: ", v.Name)
        fmt.Println("YN: ", v.YN)
        fmt.Println("Rank: ", v.Rank)
        row = sheet.AddRow()
        cell = row.AddCell()
        cell.SetValue(v.ID)
        cell = row.AddCell()
        cell.SetString(v.Name)
        cell = row.AddCell()
        cell.SetString(v.YN)
        cell = row.AddCell()
        cell.SetValue(v.Rank)
    }
    fmt.Println("TEST1")
    // Save xlsx file by the given path.
    err = file.Save(filepath.FromSlash(filepath.FromSlash("data/"+user+"/submit.xlsx")))
    if err != nil {
        fmt.Println(err)
    } else {
        fmt.Println("Saved Excel file")
    }
}

func getResults(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("getResults\n")

    files, _ := filepath.Glob("data/*/submit.js")
    resultrows := make(map[int]ResultRow)
    for _, v := range files {
        fmt.Printf("file "+v+"\n")
        data, _ := ioutil.ReadFile(v)
	stringSlice := strings.Split(v, "/")
	user := stringSlice[1]
        fmt.Printf("user "+user+"\n")
        var datarows []Row
        _ = json.Unmarshal([]byte(data), &datarows)
        for _, r := range datarows {
	    var rr ResultRow
	    if value, ok := resultrows[r.ID]; ok {
	       rr = value
	    } else {
               rr = ResultRow{Name:r.Name, YCount:0, NCount:0, YVotes:[]string{}, NVotes:[]string{}, RankTotal:0}
	    }
            if strings.ToUpper(r.YN) == "Y" {
                rr.YCount++
                rr.YVotes = append(rr.YVotes, user)
	    } else {
                rr.NCount++
                rr.NVotes = append(rr.NVotes, user)
	    }
            rr.RankTotal += r.Rank
	    resultrows[r.ID] = rr
	}
    }
    for k, v := range resultrows {
        fmt.Printf("getResults\n")
	fmt.Printf("key %d %+v %.2f\n", k, v, float64(v.RankTotal)/float64(len(files)))

    }
}

