import os
import subprocess
import socket

name = "gobuildslave"

current_hash = ""
for line in os.popen("md5sum " + name).readlines():
    current_hash = line.split(' ')[0]
    
# Move the old version over
for line in os.popen('cp ' + name + ' old' + name).readlines():
    print line.strip()

# Rebuild
for line in os.popen('go build').readlines():
    print line.strip()

size_1 = os.path.getsize('./old' + name)
size_2 = os.path.getsize('./' + name)

lines = os.popen('ps -ef | grep ' + name).readlines()
running = False
for line in lines:
    if "./" + name in line:
        running = True

new_hash = ""
for line in os.popen("md5sum " + name).readlines():
    new_hash = line.split(' ')[0]

if size_1 != size_2 or new_hash != current_hash or not running:
    if not running:
        for line in os.popen('go get github.com/brotherlogic/buildserver/buildserver_cli'):
            pass
        for line in os.popen('buildserver_cli crash gobuildslave version out.txt'):
            pass
    else:
        try:
            os.popen('go get github.com/brotherlogic/versionserver/versionserver_cli')
            os.popen('versionserver_cli guard ' + name)
        except IOError:
            os.Exit(0)
        for line in os.popen('echo "" > out.txt').readlines():
            pass
        for line in os.popen('killall ' + name).readlines():
            pass

    subprocess.Popen(['./' + name])
