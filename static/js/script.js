// hi! this script is what automatically recieves when things are finished.

let socket = new WebSocket("ws://edge.figgyc.uk/socket")

socket.addEventListener("open", (e) => {
    if (localStorage.getItem("id0") != null) {
        // send intro packet, if we get anything back then stuff will happen
        socket.send(JSON.stringify({
            id0: localStorage.getItem("id0")
        }))
    }
})

socket.addEventListener("message", (e) => {
    //console.log("hey!", e.data, JSON.parse(e.data).status)
    if (JSON.parse(e.data).status == "friendCodeAdded") {
        /* 
            Step 2: tell the user to add the bot
        */
        document.getElementById("collapseOne").classList.remove("show")
        document.getElementById("collapseTwo").classList.add("show")
    }
    if (JSON.parse(e.data).status == "friendCodeProcessing") {
        document.getElementById("fcProgress").style.display = "block"
    }
    if (JSON.parse(e.data).status == "friendCodeInvalid") {
        document.getElementById("fcProgress").style.display = "none"
        document.getElementById("fcError").style.display = "block"
        document.getElementById("beginButton").disabled = false        
    }
    if (JSON.parse(e.data).status == "movablePart1") {
        /*
            Step 3: ask user if they or worker wants to BF
            continue button
            downloadPart1 a
        */
        document.getElementById("collapseTwo").classList.remove("show")
        document.getElementById("collapseThree").classList.add("show")
        document.getElementById("downloadPart1").href = "/part1/" + localStorage.getItem("id0")
    }
    if (JSON.parse(e.data).status == "done") {
        /* 
            Step 5: done! 
            downloadMovable a
        */
        document.getElementById("collapseFour").classList.remove("show")
        document.getElementById("collapseFive").classList.add("show")
        document.getElementById("downloadMovable").href = "/movable/" + localStorage.getItem("id0")
    }
})

/*
    Step 1: gather user data
    friendCode input box
    id0 input box
    beginButton button
*/
document.getElementById("beginButton").addEventListener("click", (e) => {
    e.preventDefault()
    document.getElementById("fcError").style.display = "none"
    document.getElementById("beginButton").disabled = true
    socket.send(JSON.stringify({
        friendCode: document.getElementById("friendCode").value,
        id0: document.getElementById("id0").value,
    }))
    localStorage.setItem("id0", document.getElementById("id0").value)
})

/*
    Step 4: wait for BF
    continue button
*/
document.getElementById("continue").addEventListener("click", () => {
    //document.getElementById("").setAttribute()
    socket.send(JSON.stringify({
        request: "bruteforce",
        id0: localStorage.getItem("id0"),
    }))
    document.getElementById("collapseThree").classList.remove("show")
    document.getElementById("collapseFour").classList.add("show")
})