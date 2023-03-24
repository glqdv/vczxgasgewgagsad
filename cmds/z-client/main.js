const { app, BrowserWindow } = require('electron')
// include the Node.js 'path' module at the top of your file
const path = require('path')
const log = require('electron-log')
const process = require('process')
const os = require('os');
const { exit } = require('process');
const controller = new AbortController();
const { signal } = controller;
const isMac = os.platform() === "darwin";
const isWindows = os.platform() === "win32";


// const {spawn} = require('child_process')
log.transports.file.level = true;
const createWindow = () => {
  const win = new BrowserWindow({
    width: 1800,
    height: 1600,
    resizable: true,
    webPreferences: {
        preload: path.join(__dirname, 'preload.js')
    }
  })
  if (isMac){
    let execPath = path.join(process.resourcesPath, "app","z-client")
    log.info(`execpath : ${execPath}`);
    const cmd = require('child_process').spawn(execPath, ['-http', '-no-open'], {
      signal:signal,
      env:process.env
    });
    
    cmd.stdout.on('data', (data) => {
      if (data.indexOf(`listen tcp 0.0.0.0:35555: bind: address already in use`) > -1){
        // alert("z-client is open already!!!");
        // exit(1);
      }
      console.log(`stdout: ${data}`);
      log.info(`stdout: ${data}`)
    });
    
    cmd.stderr.on('data', (data) => {
      console.error(`stderr: ${data}`);
      log.error(`stdout: ${data}`)
    });
    
    cmd.on('close', (code) => {
      console.log(`child process exited with code ${code}`);
      log.info(`child process exited with code ${code}`);
    });
  
  }else if (isWindows){
    let execPath = path.join(process.resourcesPath, "app","z-client.exe")
    log.info(`execpath : ${execPath}`);
    const cmd = require('child_process').spawn(execPath, ['-http', '-no-open'], {
      signal:signal,
      env:process.env
    });

    cmd.stdout.on('data', (data) => {
          console.log(`stdout: ${data}`);
          log.info(`stdout: ${data}`)
    });
    
    cmd.stderr.on('data', (data) => {
      console.error(`stderr: ${data}`);
      log.error(`stdout: ${data}`)
    });
    
    cmd.on('close', (code) => {
      console.log(`child process exited with code ${code}`);
      log.info(`child process exited with code ${code}`);
    });
  }

  setTimeout(_=>{
    win.loadURL("http://localhost:35555")
  },1000)

  
}

app.whenReady().then(() => {
  createWindow()
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow()
  })

})

app.on('window-all-closed', () => {
  // cmd.exit()
  try {
    controller.abort()  
  } catch (error) {
    
  }
  
  if (process.platform !== 'darwin') app.quit()

})
