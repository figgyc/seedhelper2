#!/usr/bin/env python3

import os
import requests
import os.path
import getpass
import sys
import signal
import time
import re
import glob
import subprocess
import datetime
import io

s = requests.Session()
baseurl = "https://seedhelper.figgyc.uk"
currentid = ""
currentVersion = "1.8"

# https://stackoverflow.com/a/16696317 thx
def download_file(url, local_filename):
    # NOTE the stream=True parameter
    r = requests.get(url, stream=True)
    with open(local_filename, 'wb') as f:
        for chunk in r.iter_content(chunk_size=1024):
            if chunk: # filter out keep-alive new chunks
                f.write(chunk)
                #f.flush() commented by recommendation from J.F.Sebastian
    return local_filename


print("Checking for updates...")
r0 = s.get(baseurl + "/static/autolauncher_version")
if r0.text != currentVersion:
    print("Updating")
    download_file(baseurl + "/static/seedminer_autolauncher.py", "seedminer_autolauncher.py")
    os.system('"' + sys.executable + '" seedminer_autolauncher.py')

print("Updating seedminer db...")
os.system('"' + sys.executable + '" seedminer_launcher3.py update-db')

#username = input("Username: ")
#password = getpass.getpass("Password: ")
#print("Logging in...")
#r = s.post(baseurl + "/login", data={'username': username, 'password': password})
#print(r.url)
#if r.url == baseurl + '/home':
#    print("Login successful")
#else:
#    print("Login fail")
#    sys.exit(1)

def signal_handler(signal, frame):
        print('Exiting...')
        if currentid != "":
            cancel = input("Kill job, requeue, or do nothing? [k/r/N]")
            p = "x"
            if cancel.lower() == "r":
                p = "n"
            if cancel.lower() == "k":
                p = "y"
            if p != "x":
                s.get(baseurl + "/cancel/" + currentid + "?kill=" + p)
                sys.exit(0)
        else:
            sys.exit(0)
signal.signal(signal.SIGINT, signal_handler)



while True:
    print("Finding work...")
    try:
        r = s.get(baseurl + "/getwork")
    except:
        print("Error. Waiting 30 seconds...")
        time.sleep(30)
        continue        
    if r.text == "nothing":
        print("No work. Waiting 30 seconds...")
        time.sleep(30)
    else:
        currentid = r.text
        r2 = s.get(baseurl + "/claim/" + currentid)
        if r2.text == "error":
            print("Device already claimed, trying again...")
        else:
            print("Downloading part1 for device " + currentid)
            download_file(baseurl + '/part1/' + currentid, 'movable_part1.sed')
            print("Bruteforcing " + str(datetime.datetime.now()))
            process = subprocess.Popen([sys.executable, "seedminer_launcher3.py", "gpu"], creationflags=0x00000200)#, stdout=subprocess.PIPE, universal_newlines=True)
            timer = 0
            #stdout = open(process.stdout)
            while process.poll() == None:
                # we need to poll for kill more often then we check server because we would waste up to 30 secs after finish
                timer = timer + 1
                time.sleep(1)
                #line = process.stdout.read()
                #print(line)
                #if "offset:250" in line:
                #    print("Job taking too long, killing...")
                #    s.get(baseurl + "/cancel/" + currentid)
                #    subprocess.call(['taskkill', '/F', '/T', '/IM', 'bfcl.exe'])
                #    break
                if timer % 30 == 0:
                    r3 = s.get(baseurl + "/check/" + currentid)
                    if r3.text != "ok":
                        print("Job cancelled or expired, killing...")
                        # process.kill() broke
                        subprocess.call(['taskkill', '/F', '/T', '/IM', 'bfcl.exe'])
                        currentid = ""
                        print("press ctrl-c to quit")
                        time.sleep(5)
                        continue
            #os.system('"' + sys.executable + '" seedminer_launcher3.py gpu')
            if os.path.isfile("movable.sed"):
                print("Uploading")
                # seedhelper2 has no msed database but we upload these anyway so zoogie can have them
                list_of_files = glob.glob('msed_data_*.bin') # * means all if need specific format then *.csv
                latest_file = max(list_of_files, key=os.path.getctime)
                ur = s.post(baseurl + '/upload/' + currentid, files={'movable': open('movable.sed', 'rb'), 'msed': open(latest_file, 'rb')})
                print(ur)
                if ur.text == "success":
                    print("Upload succeeded!")
                    os.remove("movable.sed")
                    os.remove(latest_file)
                    currentid = ""
                    print("press ctrl-c to quit")
                    time.sleep(5)
                else:
                    print("Upload failed!")
                    sys.exit(1)

