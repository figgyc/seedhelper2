#!/usr/bin/env python

# system modules
import asyncio, sys, json, time, subprocess, os, os.path, glob, signal, re

try:
    import aiohttp
except ImportError:
    print('The new seedhelper script uses aiohttp. Run "pip install aiohttp" in an admin command prompt')

currentversion = "3.0"
enableupdater = False
baseurl = "https://seedhelper.figgyc.uk"
chunk_size = 1024^2
config = {}
id0 = ""
banMsg = "You have been banned from Seedhelper. This is probably because your script is glitching out. If you think you should be unbanned then find figgyc on Discord."
exitnextflag = False
writeflag = True
killflag = 0
offsetre = re.compile('offset:([\-\d]+)')

try:
    with open('config.json', 'r') as file: 
        config = json.load(file)
except Exception:
    open('config.json', 'a').close()

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
                if version == banMsg:
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
                        with open('config.json', 'w') as file:
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
            process = await asyncio.create_subprocess_exec(sys.executable, 'seedminer_launcher3.py', 'gpu', stdout=asyncio.subprocess.PIPE, cwd=os.getcwd(), stdin=asyncio.subprocess.PIPE)
            while process.returncode == None:
                print(process.returncode)
                data, _ = await process.communicate()
                line = data.decode('ascii')
                sys.stdout.write(line)
                sys.stdout.flush()
                if 'offset:10' in line:
                    process.kill()
                    timeB = time.time()
                    break
            if timeA + 100 < timeB or process.returncode != 0:
                print("Your computer is not powerful enough to help Seedhelper. Sorry!")
                return
            else:
                with open('config.json', 'w') as file:
                    config["benchmarked"] = True
                    json.dump(config, file)
        while exitnextflag == False:
            sys.stdout.write("\rSearching for work...          ")
            async with session.get(baseurl + '/getwork') as resp:
                text = await resp.text()
                if text == banMsg:
                    print(text)
                    return
                if text == "nothing":
                    sys.stdout.write("\rNo work, waiting 10 seconds...")
                    time.sleep(10)
                    continue
                id0 = text
                print("Mining " + id0)
                try:
                    async with session.get(baseurl + '/claim/' + id0) as resp:
                        text = await resp.text()
                        if text == "error":
                            print("Claim failed, probably someone else got it first.")
                            time.sleep(10)
                            continue
                        await download(session, baseurl + '/part1/' + id0, 'movable_part1.sed')
                        process = await asyncio.create_subprocess_exec(sys.executable, 'seedminer_launcher3.py', 'gpu', stdout=asyncio.subprocess.PIPE, cwd=os.getcwd(), stdin=asyncio.subprocess.PIPE)
                        n = 400
                        while process.returncode == None:
                            if killflag != 0:
                                async with session.get(baseurl + '/cancel/' + id0 + '?kill=' + ('y' if killflag == 1 else 'n')) as resp:
                                    text = await resp.text()
                                    if text == "error":
                                        print("Cancel error")
                                        continue
                                    else: 
                                        print("Killed/requeued.")
                                    id0 = ''
                                    print('Press Ctrl-C again to quit or wait to find another job')
                                    time.sleep(5)
                                    continue
                            data = await process.stdout.readuntil(b'\r')
                            line = data.decode('ascii')
                            if writeflag:
                                sys.stdout.write(line)
                                sys.stdout.flush()
                            if 'New3DS msed' in line:
                                n = 200
                            offset = int(offsetre.match(line).group(1))
                            if offset != None:
                                if offset >= n:
                                    process.kill()
                                    break
                                if offset % 5 == 0:
                                    async with session.get(baseurl + '/check/' + id0) as resp:
                                        text = await resp.text()
                                        if text == "error":
                                            print('Job expired, killing...')
                                            process.kill()
                                            break

                                    
                            
                        if returncode != 0:
                            raise Exception("Process returncode not 0")
                        
                        if os.path.isfile("movable.sed"):
                            print("Uploading...")
                            list_of_files = glob.glob('msed_data_*.bin')
                            latest_file = max(list_of_files, key=os.path.getctime)
                            async with session.post(baseurl + '/upload/' + id0, data={'movable': open('movable.sed', 'rb'), 'msed': open(latest_file, 'rb')}) as resp:
                                text = await resp.text()
                                if text == 'success':
                                    print('Upload succeeded!')
                                    os.remove('movable.sed')
                                    os.remove(latest_file)
                                    id0 = ''
                                    time.sleep(5)
                                else:
                                    raise Exception("Upload failed")
                        else:
                            raise FileNotFoundError("movable.sed is not generated")
                except Exception as e:
                    print("Error, cancelling...")
                    print(e)
                    async with session.get(baseurl + '/cancel/' + id0) as resp:
                        text = await resp.text()
                        if text == "error":
                            print("Cancel error")
                        else: 
                            print("Cancelled")
                        time.sleep(10)
                        continue

                time.sleep(10)
            
def signal_handler(signal, frame):
    if id0 != '':
        writeflag = False
        letter = input("Control-C pressed. Type an option from [r]equeue job (default), [k]ill job, [c]ancel (do nothing), or [e]xit after this job: ").lower()
        if letter == '':
            letter = 'k'
        if letter == 'k':
            killflag = 1
        elif letter == 'r':
            killflag = 2
        elif letter == 'e':
            exitnextflag = True
        elif letter == 'c':
            return
        print("Okay.")
        writeflag = True
    else:
        print('Exiting...')
        sys.exit(0)


signal.signal(signal.SIGINT, signal_handler) 

# win32 needs the proactor loop for asyncio subprocesses
if sys.platform == 'win32':
    loop = asyncio.ProactorEventLoop()
    asyncio.set_event_loop(loop)

loop = asyncio.get_event_loop()
loop.run_until_complete(main())


loop.close()
