#!/usr/bin/env python

import aiohttp, asyncio, sys, subprocess, json, time

currentversion = "3.0"
enableupdater = False
baseurl = "https://seedhelper.figgyc.uk"
chunk_size = 1024^2
config = {}
id0 = ""


try:
    with open('config', 'r') as file: 
        config = json.load(file)
except Exception:
    pass

async def download(session, url, filename):
    async with session.get(url) as resp:
        with open(filename, 'wb') as fd:
            while True:
                chunk = await resp.content.read(chunk_size)
                if not chunk:
                    break
                fd.write(chunk)

async def main():
    async with aiohttp.ClientSession() as session:
        if enableupdater:
            print("Updating...")
            async with session.get(baseurl + '/static/autolauncher_version') as resp:
                version = await resp.text()
                if version == "You have been banned from Seedhelper. This is probably because your script is glitching out. If you think you should be unbanned then find figgyc on Discord.":
                    print(version)
                    return
                if version != currentversion:
                    print("Updating...")
                    await download(session, baseurl + '/static/seedminer_autolauncher.py', 'seedminer_autolauncher.py')
                    subprocess.run([sys.executable, 'seedminer_autolauncher.py'])
                    return
        # main code
        if not config.get('nameset', False):
            while True:
                name = input('Type a name to set, ideally a Discord name, which will appear on leaderboard and may be used to contact you if you are having issues: ')
                async with session.get(baseurl + '/setname', params={'name': name}) as resp:
                    message = await resp.text()
                    if message != "success":
                        print(message)
                    else:
                        with open('config', 'r') as file:
                            config["nameset"] = True
                            json.dump(config, file)
                        break
        print("Updating Seedminer database:")
        subprocess.run([sys.executable, 'seedminer_launcher3.py', 'update-db'])
        if not config.get('benchmarked', False):
            print("Benchmarking")
            timeA = time.time()
            timeB = time.time() + 200
            await download(session, baseurl + '/static/impossible_part1.sed', 'movable_part1.sed')
            process = subprocess.Popen([sys.executable, 'seedminer_launcher3.py', 'gpu'], stdout=subprocess.PIPE, stdin=subprocess.PIPE, universal_newlines=True)
            while process.poll() == None:
                line = process.stdout.readline()
                sys.stdout.write(line)
                if 'offset:10' in line:
                    process.kill()
                    timeB = time.time()
                    break
            if timeA + 100 < timeB:
                print("Your computer is not powerful enough to help Seedhelper. Sorry!")
                return
            else:
                with open('config', 'r') as file:
                    config["benchmarked"] = True
                    json.dump(config, file)
        print("Searching for work...")
        while True:
            async with session.get(baseurl + '/getwork') as resp:
                text = await resp.text()
                if text == "You have been banned from Seedhelper. This is probably because your script is glitching out. If you think you should be unbanned then find figgyc on Discord.":
                    print(text)
                    return
                if text == "nothing":
                    sys.stdout.write("\rNo work, waiting 10 seconds...")
                    sys.sleep(10)
                    continue
                id0 = text
                print(id0)
                sys.sleep(10)
            
        

loop = asyncio.get_event_loop()
loop.run_until_complete(main())
loop.close()
