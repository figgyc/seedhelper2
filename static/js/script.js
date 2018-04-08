// hi! this script is what automatically recieves when things are finished.

//thx
// Converts an ArrayBuffer directly to base64, without any intermediate 'convert to string then
// use window.btoa' step. According to my tests, this appears to be a faster approach:
// http://jsperf.com/encoding-xhr-image-data/5

/*
MIT LICENSE

Copyright 2011 Jon Leighton

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

function base64ArrayBuffer(arrayBuffer) {
    var base64    = ''
    var encodings = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
  
    var bytes         = new Uint8Array(arrayBuffer)
    var byteLength    = bytes.byteLength
    var byteRemainder = byteLength % 3
    var mainLength    = byteLength - byteRemainder
  
    var a, b, c, d
    var chunk
  
    // Main loop deals with bytes in chunks of 3
    for (var i = 0; i < mainLength; i = i + 3) {
      // Combine the three bytes into a single integer
      chunk = (bytes[i] << 16) | (bytes[i + 1] << 8) | bytes[i + 2]
  
      // Use bitmasks to extract 6-bit segments from the triplet
      a = (chunk & 16515072) >> 18 // 16515072 = (2^6 - 1) << 18
      b = (chunk & 258048)   >> 12 // 258048   = (2^6 - 1) << 12
      c = (chunk & 4032)     >>  6 // 4032     = (2^6 - 1) << 6
      d = chunk & 63               // 63       = 2^6 - 1
  
      // Convert the raw binary segments to the appropriate ASCII encoding
      base64 += encodings[a] + encodings[b] + encodings[c] + encodings[d]
    }
  
    // Deal with the remaining bytes and padding
    if (byteRemainder == 1) {
      chunk = bytes[mainLength]
  
      a = (chunk & 252) >> 2 // 252 = (2^6 - 1) << 2
  
      // Set the 4 least significant bits to zero
      b = (chunk & 3)   << 4 // 3   = 2^2 - 1
  
      base64 += encodings[a] + encodings[b] + '=='
    } else if (byteRemainder == 2) {
      chunk = (bytes[mainLength] << 8) | bytes[mainLength + 1]
  
      a = (chunk & 64512) >> 10 // 64512 = (2^6 - 1) << 10
      b = (chunk & 1008)  >>  4 // 1008  = (2^6 - 1) << 4
  
      // Set the 2 least significant bits to zero
      c = (chunk & 15)    <<  2 // 15    = 2^4 - 1
  
      base64 += encodings[a] + encodings[b] + encodings[c] + '='
    }
    
    return base64
  }
  //

let socket = new WebSocket("wss://" + location.host + "/socket")
let force = "no"

socket.addEventListener("open", (e) => {
    if (localStorage.getItem("id0") != null) {
        // send intro packet, if we get anything back then stuff will happen
        socket.send(JSON.stringify({
            id0: localStorage.getItem("id0")
        }))
    }
})

socket.addEventListener("close", () => {setTimeout(() => {window.location.reload(true)}, 10000)})
socket.addEventListener("error", () => {setTimeout(() => {window.location.reload(true)}, 10000)})

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
        document.getElementById("collapseOne").classList.remove("show")
        document.getElementById("collapseTwo").classList.remove("show")
        document.getElementById("collapseThree").classList.add("show")
        document.getElementById("downloadPart1").href = "/part1/" + localStorage.getItem("id0")
    }
    if (JSON.parse(e.data).status == "done") {
        /* 
            Step 5: done! 
            downloadMovable a
        */
        document.getElementById("collapseOne").classList.remove("show")
        document.getElementById("collapseTwo").classList.remove("show")
        document.getElementById("collapseThree").classList.remove("show")
        document.getElementById("collapseFour").classList.remove("show")
        document.getElementById("collapseFive").classList.add("show")
        document.getElementById("downloadMovable").href = "/movable/" + localStorage.getItem("id0")
    }
    if (JSON.parse(e.data).status == "flag") {
        /* 
            Step -1: flag
        */
        document.getElementById("collapseOne").classList.add("show")
        document.getElementById("collapseTwo").classList.remove("show")
        document.getElementById("collapseThree").classList.remove("show")
        document.getElementById("collapseFour").classList.remove("show")
        document.getElementById("collapseFive").classList.remove("show")
        document.getElementById("fcError").style.display = "block"
        document.getElementById("fcError").innerText = "Your movable.sed took to long to bruteforce. This is most likely because your ID0 was incorrect. Please make sure it is correct by asking for help."
    }
    if (JSON.parse(e.data).status == "bruteforcing") {
        /* 
            Step 4.1
        */
        document.getElementById("collapseOne").classList.remove("show")
        document.getElementById("collapseTwo").classList.remove("show")
        document.getElementById("collapseThree").classList.remove("show")
        document.getElementById("collapseFour").classList.add("show")
        document.getElementById("collapseFive").classList.remove("show")
        document.getElementById("bfProgress").classList.add("bg-warning")
    }
    if (JSON.parse(e.data).status == "couldBeID1") {
        document.getElementById("fcProgress").style.display = "none"
        document.getElementById("fcWarning").style.display = "block"
        document.getElementById("beginButton").disabled = false
        force = "yes"
    }
})

/*
    Step 0?: parse preprovided part1
    uploadP1 a
    p1file input[type=file]
*/
document.getElementById("uploadp1").addEventListener("click", (e) => {
    let fileInput = document.getElementById("p1file")    
    fileInput.click()
})

document.getElementById("p1file").addEventListener("change", (e) => {
    let fileInput = e.target
    let fileList = fileInput.files
    if (fileList.length == 1 && fileList[0].size == 0x1000) {
        let file = fileInput.files[0]
        let fileReader = new FileReader()
        fileReader.readAsArrayBuffer(file)
        fileReader.addEventListener("loadend", () => {
            let arrayBuffer = fileReader.result
            let lfcsBuffer = arrayBuffer.slice(0, 8)
            let lfcsArray = new Uint8Array(lfcsBuffer)
            if (lfcsBuffer == new Uint8Array(8)) {
                alert("part1 is invalid")
                return
            }
            document.getElementById("part1b64").value = base64ArrayBuffer(lfcsBuffer)
            let id0Buffer = arrayBuffer.slice(0x10, 0x10+32)
            let id0Array = new Uint8Array(id0Buffer)
            document.getElementById("friendCode").disabled = true
            document.getElementById("friendCode").value = "movable_part1 provided"
            let textDecoder = new TextDecoder()
            let id0String = textDecoder.decode(id0Array)
            console.log(id0String,  btoa(id0String), id0String.length)
            if (btoa(id0String) != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") { // non blank, if id0 is injected with seedminer_helper
                let id0Input = document.getElementById("id0")
                id0Input.disabled = true
                id0Input.value = id0String
            }
        })
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
    document.getElementById("friendCode").value = document.getElementById("friendCode").value.replace(/-/g, "")
    document.getElementById("fcError").style.display = "none"
    document.getElementById("beginButton").disabled = true
    localStorage.setItem("id0", document.getElementById("id0").value)
    if (document.getElementById("part1b64").value != "") {
        socket.send(JSON.stringify({
            part1: document.getElementById("part1b64").value,
            defoID0: force,
            id0: document.getElementById("id0").value,
        }))
    } else {
        socket.send(JSON.stringify({
            friendCode: document.getElementById("friendCode").value,
            id0: document.getElementById("id0").value,
            defoID0: force
        }))
    }
})

/*
    Step 4: wait for BF
    continue button
*/
document.getElementById("continue").addEventListener("click", (e) => {
    e.preventDefault()
    //document.getElementById("").setAttribute()
    socket.send(JSON.stringify({
        request: "bruteforce",
        id0: localStorage.getItem("id0"),
    }))
    document.getElementById("collapseThree").classList.remove("show")
    document.getElementById("collapseFour").classList.add("show")
    document.getElementById("id0Fill").innerText = localStorage.getItem("id0")
})

// anti queue clogging
document.querySelector(".disableButton").addEventListener("click", (e) => {
    document.getElementById("continue").disabled = true
    document.getElementById("disableMessage").style.display = "block"
})

document.getElementById("enableButton").addEventListener("click", (e) => {
    document.getElementById("continue").disabled = false
    document.getElementById("disableMessage").style.display = "none"
})

/*
    cancel task
*/

function cancel (e) {
    e.preventDefault()
    document.getElementById("cancelButton").disabled = true
    document.getElementById("downloadPart1").click()
    socket.send(JSON.stringify({
        request: "cancel",
        id0: document.getElementById("id0").value,
    }))
    document.getElementById("collapseFour").classList.remove("show")
    document.getElementById("collapseOne").classList.add("show")
    localStorage.clear();
    location.reload(true);
}

document.getElementById("cancelButton").addEventListener("click", cancel)
document.getElementById("cancelButton1").addEventListener("click", cancel)
document.getElementById("cancelButton2").addEventListener("click", cancel)
document.getElementById("cancelButton3").addEventListener("click", cancel)