# -*- coding: utf-8 -*-
import time
import sys
import json
from smbus import SMBus
import requests
from concurrent.futures.thread import ThreadPoolExecutor

b = SMBus(1)

#Device I2C Arress
LCD_ADDRESS   =  (0x7c>>1)

LCD_CLEARDISPLAY = 0x01
LCD_RETURNHOME = 0x02
LCD_ENTRYMODESET = 0x04
LCD_DISPLAYCONTROL = 0x08
LCD_CURSORSHIFT = 0x10
LCD_FUNCTIONSET = 0x20
LCD_SETCGRAMADDR = 0x40
LCD_SETDDRAMADDR = 0x80

#flags for display entry mode
LCD_ENTRYRIGHT = 0x00
LCD_ENTRYLEFT = 0x02
LCD_ENTRYSHIFTINCREMENT = 0x01
LCD_ENTRYSHIFTDECREMENT = 0x00

#flags for display on/off control
LCD_DISPLAYON = 0x04
LCD_DISPLAYOFF = 0x00
LCD_CURSORON = 0x02
LCD_CURSOROFF = 0x00
LCD_BLINKON = 0x01
LCD_BLINKOFF = 0x00

#flags for display/cursor shift
LCD_DISPLAYMOVE = 0x08
LCD_CURSORMOVE = 0x00
LCD_MOVERIGHT = 0x04
LCD_MOVELEFT = 0x00

#flags for function set
LCD_8BITMODE = 0x10
LCD_4BITMODE = 0x00
LCD_2LINE = 0x08
LCD_1LINE = 0x00
LCD_5x8DOTS = 0x00


class LCD1602:
  def __init__(self, col, row):
    self._row = row
    self._col = col
    self._showfunction = LCD_4BITMODE | LCD_1LINE | LCD_5x8DOTS;
    self.begin(self._row,self._col)


  def command(self,cmd):
    b.write_byte_data(LCD_ADDRESS,0x80,cmd)

  def write(self,data):
    b.write_byte_data(LCD_ADDRESS,0x40,data)

  def setCursor(self,col,row):
    if(row == 0):
      col|=0x80
    else:
      col|=0xc0;
    self.command(col)

  def clear(self):
    self.command(LCD_CLEARDISPLAY)
    time.sleep(0.02)
  def printout(self,arg):
    if(isinstance(arg,int)):
      arg=str(arg)

    for x in bytearray(arg,'utf-8'):
      self.write(x)


  def display(self):
    self._showcontrol |= LCD_DISPLAYON
    self.command(LCD_DISPLAYCONTROL | self._showcontrol)


  def begin(self,cols,lines):
    if (lines > 1):
        self._showfunction |= LCD_2LINE

    self._numlines = lines
    self._currline = 0

    time.sleep(0.05)


    # Send function set command sequence
    self.command(LCD_FUNCTIONSET | self._showfunction)
    #delayMicroseconds(4500);  # wait more than 4.1ms
    time.sleep(0.005)
    # second try
    self.command(LCD_FUNCTIONSET | self._showfunction);
    #delayMicroseconds(150);
    time.sleep(0.005)
    # third go
    self.command(LCD_FUNCTIONSET | self._showfunction)
    # finally, set # lines, font size, etc.
    self.command(LCD_FUNCTIONSET | self._showfunction)
    # turn the display on with no cursor or blinking default
    self._showcontrol = LCD_DISPLAYON | LCD_CURSOROFF | LCD_BLINKOFF
    self.display()
    # clear it off
    self.clear()
    # Initialize to default text direction (for romance languages)
    self._showmode = LCD_ENTRYLEFT | LCD_ENTRYSHIFTDECREMENT
    # set the entry mode
    self.command(LCD_ENTRYMODESET | self._showmode);



Exe = ThreadPoolExecutor(10)
Used = {}
def button(pin_num, callback):
  try:
    with open("/sys/class/gpio/unexport","w") as fp:
      fp.write(str(pin_num))
      print("Close pin :",pin_num)
    print("Open pin :",pin_num)
  except :
    pass
  try:
    
    with open("/sys/class/gpio/export","w") as fp:
      fp.write(str(pin_num))
    with open("/sys/class/gpio/gpio"+str(pin_num) + "/direction","w") as fp:
      fp.write("in")
    last_push = time.time()
    st_stat = False
    ww = "/sys/class/gpio/gpio"+str(pin_num) + "/value"
    print("Init :" + ww)
    while 1:
      time.sleep(0.2)
      with open(ww,"r") as fp:
        if fp.read().strip() == "1":
          if not st_stat:
            last_push = time.time()
            st_stat = True
            
        else:
          if st_stat :
            st_stat = False
            call_time = time.time() - last_push
            callback(call_time)
  except Exception as e:
    print(e)
  finally:
    with open("/sys/class/gpio/unexport","w") as fp:
      fp.write(str(pin_num))
    print("Close pin :",pin_num)


class Controller:
  def __init__(self):
    self.lcd=LCD1602(16,2)
    self.one = "Hello world"
    self.two = ".(* *). ?"

  def loop(self):
    lcd = self.lcd
    try:
      l = time.time()
      while True:
      # set the cursor to column 0, line 1
        lcd.setCursor(0, 0)
      # print the number of seconds since reset:
      # print the number of seconds since reset:
        lcd.printout(self.one)
        lcd.setCursor(0, 1)
        lcd.printout(self.two)
        time.sleep(1)
        if time.time() - l > 4 :
          Exe.submit(self.get_state)
    except(KeyboardInterrupt):
      lcd.clear()
      del lcd

  def _delay_msg(self, t,msg):
    time.sleep(t)
    self.show(msg)

  def show(self, msg, wait=None):
    one= self.one
    two = self.two
    if wait != None:
      Exe.submit(self._delay_msg, wait, one +"\n"+two)
      self.show(msg)

    else:
      if "\n" in msg:
        on,tw = msg.split("\n",1)
        self.one = on[:16]
        self.two = tw[:16]
      else:
        if len(msg) > 16:
          self.one = msg[:16]
          self.two = msg[16:32]
        else:
          self.one = msg


  def regist_btn(self, pin_num,callback):
    if Used.get(pin_num) == None:
      print("try add pin:",pin_num)
      Exe.submit(button, pin_num,callback)
      Used[pin_num] = True

  def switch(self, call_time):
    if call_time > 0.5:
      print("Long time --- to close")
      self.show("Switch To      \nChina/World               ")
      try:
        res = requests.post("http://127.0.0.1:35555/z-route",json.dumps({
          "op":"open/close",
        })).json()
        self.show(res["msg"])
      except Exception as e:
        self.show(str(e), wait=4)
    else:
      self.show("Switch next rotue\n the fast route mode")
      try:
        res = requests.post("http://127.0.0.1:35555/z-api",json.dumps({
          "op":"switch",
        })).json()
        self.show(res["msg"])
      except Exception as e:
        self.show(str(e), wait=4)

  def get_state(self):
    try:
      res = requests.post("http://127.0.0.1:35555/z-api",json.dumps({
        "op":"check",
      })).json()
      e = "X"
      if res["msg"]["mode"] == "route":
        e = ">"
      if e == "X":
        self.show(e+res["msg"]["running"]+"\n"+"China Net         ")
      else:
        self.show(e+res["msg"]["running"]+"\n"+res["msg"]["loc"]+"   ")
    except Exception as e:
      self.show("Please Login!\n.2.1:35555/z-login")
  
  def openClose(self,utime):
    print("Long time --- to close")
    self.show("Switch To      \nChina/World               ")
    try:
      res = requests.post("http://127.0.0.1:35555/z-route",json.dumps({
        "op":"open/close",
      })).json()
      self.show(res["msg"])
    except Exception as e:
      self.show(str(e), wait=4)

if __name__ == "__main__":
  import os
  try:
    if "python" in os.popen("ps | grep lcd-btn.py").read():
      sys.exit(0)
  except:
    pass
  con = Controller()
  con.regist_btn(26, con.switch)
  con.regist_btn(20, con.openClose)

  con.loop()