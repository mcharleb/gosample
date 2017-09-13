package main

import (
    "fmt"
    "strconv"
    "encoding/json"
    "net/http"
    "sync"
    "io/ioutil"
    "github.com/satori/go.uuid"
    "github.com/xuri/excelize"
)

var sessionStore map[string]Client
var storageMutex sync.RWMutex

type Client struct {
    loggedIn bool
    username string
}

const css = `<style>
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
`

const loginPage = `<html>
<head>
    <title>Login</title>
</head>` + css + `
<body>
    <div class="container">
    <form id="login" action="/login" method="post">
	<label><b>Username</b></label>
	<input type="input" name="user" />
	<label><b>Password</b></label>
	<input type="password" name="pin" />
        <button type="submit" />Login</button>
    </form>
    <form id="reset" action="/reset" method="post">
        <button type="submit" />Reset PIN</button>
    </form>
    </div>
</body>
</html>`

const resetPage = `<html>
<head>
    <title>Reset PIN</title>
</head>` + css + `
<body>
    <h1>Reset PIN</h1>
    <form id="pwreset" action="/reset" method="post">
        <div class="container">
	<label><b>Username</b></label>
	<input type="input" name="user" />
	<label><b>New PIN</b></label>
	<input type="password" name="pin" />
        <button type="submit" />Reset PIN</button>
        </div>
    </form>
</body>
</html>`
func main() {

    sessionStore = make(map[string]Client)
    http.Handle("/scripts/", authenticate(pageHandler{""}))
    http.Handle("/review2017", authenticate(pageHandler{"review.html"}))
    http.HandleFunc("/login", handleLogin)
    http.HandleFunc("/reset", handlePinReset)
    http.HandleFunc("/logout", handleLogout)
    http.HandleFunc("/ajax/save", save)
    http.HandleFunc("/ajax/submit", submit)
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
        bytepin, err := ioutil.ReadFile("data/"+user+"_pin.js")
        if err != nil {
            fmt.Fprint(w, resetPage)
            fmt.Printf("No pin saved\n")
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
        fmt.Fprint(w, resetPage)
        return
    }
    fmt.Printf("\n")
    var user = r.FormValue("user");
    var pin = r.FormValue("pin");
    if user == "" || pin == "" {
        fmt.Fprint(w, resetPage)
        fmt.Fprint(w, "Username or PIN cannot be empty")
        return
    }

    fmt.Printf("Reset "+user+" "+pin+"\n")
    ioutil.WriteFile("data/"+user+"_pin.js", []byte(pin), 0644)
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

func submit(w http.ResponseWriter, r *http.Request) {
    fmt.Printf("submit\n")
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
    if user != formuser {
        fmt.Printf("Error: user not provided\n")
        return
    }
    //var data = r.FormValue("data")
    //fmt.Println("data:", data)
    data := []byte(`[{"ID":1,"Name":"Oli Bob","YN":"y","Rank":1,"Notes":"a"},{"ID":2,"Name":"Mary May","YN":"y","Rank":2,"Notes":"d"},{"ID":3,"Name":"Christine Lobowski","YN":"y","Rank":3,"Notes":"d"},{"ID":4,"Name":"Brendon Philips","YN":"y","Rank":4,"Notes":"d"},{"ID":5,"Name":"Margret Marmajuke","YN":"y","Rank":5,"Notes":""}]`)
    type Row struct {
        ID  int
        Name string
        YN string
        Rank int
        Notes string
    }
    var row []Row
    err2 := json.Unmarshal(data, &row)
    if err2 != nil {
        fmt.Println("error:", err2)
    }
    xlsx := excelize.NewFile()
 // Create a new sheet.
    xlsx.NewSheet(2, "Sheet2")
    // Set value of a cell.
    xlsx.SetCellValue("Sheet2", "A1", user)
    xlsx.SetCellValue("Sheet1", "B2", 100)
    // Set active sheet of the workbook.
    xlsx.SetActiveSheet(2)
    for j, v := range row {
        fmt.Println("Saving row", j)
        fmt.Println("ID: ", v.ID)
        fmt.Println("Name: ", v.Name)
        fmt.Println("YN: ", v.YN)
        fmt.Println("Rank: ", v.Rank)
	idxstr := strconv.Itoa(j+1)
        xlsx.SetCellValue("Sheet2", "A"+idxstr, v.ID)
        xlsx.SetCellValue("Sheet2", "B"+idxstr, v.Name)
        xlsx.SetCellValue("Sheet2", "C"+idxstr, v.YN)
        xlsx.SetCellValue("Sheet2", "D"+idxstr, v.Rank)
    }
    // Save xlsx file by the given path.
    err3 := xlsx.SaveAs("data/"+user+".xlsx")
    if err3 != nil {
        fmt.Println(err3)
    }
}
