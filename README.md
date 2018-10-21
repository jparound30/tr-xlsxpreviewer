# The support tool for TestRail&trade; Testcase creation.

* Export CSV file ( for import into TestRail)
* Simple and Realtime preview of rich-text formatting text.

## Usage
```
This tool uses port 10080 for internal web server API.
After running this tool, access "http://localhost:10080" with your favarite browser.

  -input string
    	Specify file path of input [XLSX file]. (default "./testcase.xlsx")
  -output string
    	Specify file path of output [CSV file]. (default "./testcase.csv")
```



### Notice
This software uses the following libraries / frameworks.

##### golang
###### Excelize
Golang library for reading and writing Microsoft Excel&trade; (XLSX) files.  
https://github.com/360EntSecGroup-Skylar/excelize

###### Gorilla WebSocket
A WebSocket implementation for Go.  
https://github.com/gorilla/websocket

###### fsnotify
Cross-platform file system notifications for Go.  
https://github.com/fsnotify/fsnotify

##### Javascript
###### Vue.js
A progressive, incrementally-adoptable JavaScript framework for building UI on the web.  
https://vuejs.org/

###### Vuetify
Material Component Framework for Vue.js 2  
https://vuetifyjs.com/en/

###### pagedown  [/asserts/js/pagedown]
A javascript port of Markdown, as used on Stack Overflow and the rest of Stack Exchange network.  
https://github.com/StackExchange/pagedown

