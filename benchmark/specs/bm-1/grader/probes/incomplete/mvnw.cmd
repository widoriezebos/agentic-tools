@echo off
if not exist target\classes mkdir target\classes
javac --release 21 -d target\classes src\main\java\ProbeTaskrun.java || exit /b 1
copy /y probe-mode.txt target\classes\probe-mode.txt >nul
jar --create --file target\taskrun.jar --main-class ProbeTaskrun -C target\classes . || exit /b 1
