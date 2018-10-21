package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// TODO
//  全体的に整理必要...

// Excel/CSVに使うヘッダ用の各カラムの識別子
const (
	ColId             = "ID"
	ColSection        = "Section"
	ColTitle          = "Title"
	ColType           = "Type"
	ColPriority       = "Priority"
	ColPreconditions  = "Preconditions"
	ColSteps          = "Steps"
	ColExpectedResult = "Expected Result"
)

// CSV出力時のヘッダ行に使用するカラム名配列
var CsvHeaderArray = []string{
	ColId,
	ColSection,
	ColTitle,
	ColType,
	ColPriority,
	ColPreconditions,
	ColSteps,
	ColExpectedResult,
}

// TestRailのテストケース構造体
type TestCase struct {
	ID             string   `json:"ID"`
	Sections       []string `json:"Sections"`
	Title          string   `json:"Title"`
	Type           string   `json:"Type"`
	Priority       string   `json:"Priority"`
	Preconditions  string   `json:"Preconditions"`
	Steps          string   `json:"Steps"`
	ExpectedResult string   `json:"ExpectedResult"`
	Depth          int      `json:"Depth"`
}

// サブセクションを">"で結合することになっている(TestRail側)のでその変換を行う
func (tc *TestCase) Section() string {
	builder := strings.Builder{}
	for i, section := range tc.Sections {
		if i != 0 {
			builder.WriteString(">")
		}
		builder.WriteString(section)
	}
	return builder.String()
}

// CSV書き出し用に配列変換を行う
// テストケースに必要な情報だけ決まった順序で配列に書き出す
func (tc *TestCase) ToArray() []string {
	var result []string
	result = append(result, tc.ID, tc.Section(), tc.Title, tc.Type, tc.Priority, tc.Preconditions, tc.Steps, tc.ExpectedResult)
	return result
}

// 空の項目がないかを検査
func (tc *TestCase) Validate() (bool, error) {
	var invalidColumns []string
	if tc.ID == "" {
		invalidColumns = append(invalidColumns, ColId)
	}
	if tc.Title == "" {
		invalidColumns = append(invalidColumns, ColTitle)
	}
	if tc.Type == "" {
		invalidColumns = append(invalidColumns, ColType)
	}
	if tc.Priority == "" {
		invalidColumns = append(invalidColumns, ColPriority)
	}
	if tc.Preconditions == "" {
		invalidColumns = append(invalidColumns, ColPreconditions)
	}
	if tc.Steps == "" {
		invalidColumns = append(invalidColumns, ColSteps)
	}
	if tc.ExpectedResult == "" {
		invalidColumns = append(invalidColumns, ColExpectedResult)
	}
	if len(tc.Sections) != 0 {
		for _, value := range tc.Sections {
			if value == "" {
				invalidColumns = append(invalidColumns, ColSection)
				break
			}
		}
	}
	if len(invalidColumns) != 0 {
		builder := strings.Builder{}
		builder.WriteString("[")
		for _, col := range invalidColumns {
			builder.WriteString(col + ",")
		}
		e := builder.String()
		e = e[:len(e)-1] + "]"
		return false, errors.New("Empty column: " + e)
	} else {
		return true, nil
	}
}

// TestCase生成メソッド
func NewTestCase(id *string, section []string, title, typ, priority, preconditions, steps, expectedResult *string) *TestCase {
	return &TestCase{
		ID:             *id,
		Sections:       section,
		Title:          *title,
		Type:           *typ,
		Priority:       *priority,
		Preconditions:  *preconditions,
		Steps:          *steps,
		ExpectedResult: *expectedResult,
		Depth:          len(section),
	}
}

// ヘッダ行から割り出したテストケース情報の各カラムの位置情報
// 位置情報は0オリジン
type ColumnPositions struct {
	ID                  int
	foundId             bool
	Sections            []int
	Title               int
	foundTitle          bool
	Type                int
	foundType           bool
	Priority            int
	foundPriority       bool
	Preconditions       int
	foundPreconditions  bool
	Steps               int
	foundSteps          bool
	ExpectedResult      int
	foundExpectedResult bool
}

// 必要なヘッダ情報が取得できたかを検査
func (cp *ColumnPositions) isValid() bool {
	// セクションはなしでもいいことにする。
	return cp.foundId && cp.foundTitle && cp.foundType && cp.foundPriority &&
		cp.foundPreconditions && cp.foundSteps && cp.foundExpectedResult // || len(cp.Sections) != 0
}

// Excelデータのヘッダ情報(通常１行目）を解析
// ・カラム位置情報を解析
func (cp *ColumnPositions) AnalyseHeader(header []string) error {
	// 一旦初期化
	cp.ID = 0
	cp.foundId = false
	cp.Sections = []int{}
	cp.Title = 0
	cp.foundTitle = false
	cp.Type = 0
	cp.foundType = false
	cp.Priority = 0
	cp.foundPriority = false
	cp.Preconditions = 0
	cp.foundPreconditions = false
	cp.Steps = 0
	cp.foundSteps = false
	cp.ExpectedResult = 0
	cp.foundExpectedResult = false

	for pos, columnName := range header {
		switch columnName {
		case ColId:
			cp.ID = pos
			cp.foundId = true
		case ColTitle:
			cp.Title = pos
			cp.foundTitle = true
		case ColType:
			cp.Type = pos
			cp.foundType = true
		case ColPriority:
			cp.Priority = pos
			cp.foundPriority = true
		case ColPreconditions:
			cp.Preconditions = pos
			cp.foundPreconditions = true
		case ColSteps:
			cp.Steps = pos
			cp.foundSteps = true
		case ColExpectedResult:
			cp.ExpectedResult = pos
			cp.foundExpectedResult = true
		default:
			if !strings.HasPrefix(columnName, ColSection) {
				continue
			}
			cp.Sections = append(cp.Sections, pos)
		}
	}

	if !cp.isValid() {
		return errors.New("必須のヘッダがありませんよ")
	}

	return nil
}

func assetsHandler(w http.ResponseWriter, r *http.Request) {
	requestURL := r.URL.String()

	log.Printf("[REQ]\t%s\n", requestURL)

	// ローカルの./asserts/...があればそちらを優先する
	base := "assets"
	reqPath := requestURL
	if reqPath == "/" {
		reqPath = "/index.html"
	}
	file, err := os.Open(base + reqPath)

	var all []byte
	if err == nil {
		all, err = ioutil.ReadAll(file)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	} else {
		// ローカルの./asserts/がないので埋め込んだassetsを読み出す
		path := "/" + base + reqPath
		f, err := Assets.Open(path)
		if err != nil {
			log.Println("Assets:" + path + ": " + err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}
		all, _ = ioutil.ReadAll(f)
	}
	defer file.Close()

	if strings.HasSuffix(reqPath, ".html") {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
	} else if strings.HasSuffix(reqPath, ".css") {
		w.Header().Add("Content-Type", "text/css")
	} else if strings.HasSuffix(reqPath, ".js") {
		w.Header().Add("Content-Type", "application/javascript")
	} else {
		// TODO ユースケースないのでほっとく
		log.Printf("Unknown file type.")
	}
	// HeaderはWriteHeader/Writeを呼ぶ前に設定する必要がある
	w.WriteHeader(http.StatusOK)
	// WriteするとWriteHeader(200)が暗黙的に実行されるので、一番最後に実行するのがいい
	_, _ = w.Write(all)
}

func testCaseDataHandler(w http.ResponseWriter, r *http.Request) {
	requestURL := r.URL.String()
	log.Printf("[REQ]\t%s\n", requestURL)

	// 指定ファイルオープンしexcelize.Fileを取得
	xlsx, err := excelize.OpenFile(inputFile)
	if err != nil {
		log.Fatal(err)
		return
	}

	var testCases []*TestCase

	testCases, err = BuildTestCasesFromXlsx(xlsx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	// とりあえずjson変換？
	b, _ := json.Marshal(testCases)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}
	watcher := NewWatcher()
	go writer(ws, watcher)
	reader(ws)
}

func reader(ws *websocket.Conn) {
	defer func() {
		_ = ws.Close()
	}()
	ws.SetReadLimit(512)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

type NullString struct {
	String string
	Valid  bool
}
type ResponseTestCases struct {
	Err       NullString  `json:"err"`
	TestCases []*TestCase `json:"testCases"`
}

func (s NullString) MarshalJSON() ([]byte, error) {
	if s.Valid {
		return json.Marshal(s.String)
	} else {
		return []byte("null"), nil
	}
}

func writer(ws *websocket.Conn, watcher *fsnotify.Watcher) {
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		_ = ws.Close()
		_ = watcher.Close()
	}()
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("event:", event)
			if event.Op&fsnotify.Rename == fsnotify.Rename ||
				event.Op&fsnotify.Remove == fsnotify.Remove {
				// TODO workaround
				//  macOSのExcel2016はrename, 2011だとremoveが通知されるなど、Excelの保存処理自体が単なるWriteではない模様。
				//  そのためイベントがRename/Removeの場合再度同一ファイルでwatchを追加する
				time.Sleep(100 * time.Millisecond) // TODO Sleep時間は要検証 pollingするべきかもしれない
				if err := watcher.Add(inputFile); err != nil {
					log.Println(err)
					return
				}
			}
			var p []byte
			var err error

			testCases, err := ConvertExcelToJson()
			var responseTestCases ResponseTestCases
			if err != nil {
				responseTestCases = ResponseTestCases{
					Err:       NullString{String: err.Error(), Valid: true},
					TestCases: nil,
				}
			} else {
				responseTestCases = ResponseTestCases{
					Err:       NullString{String: "", Valid: false},
					TestCases: testCases,
				}
			}
			// json変換
			p, err = json.Marshal(responseTestCases)
			if err != nil {
				responseTestCases = ResponseTestCases{
					Err:       NullString{String: err.Error(), Valid: true},
					TestCases: nil,
				}
				p, _ = json.Marshal(responseTestCases)
			}

			if p != nil {
				_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
				if err := ws.WriteMessage(websocket.TextMessage, p); err != nil {
					return
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		case <-pingTicker.C:
			_ = ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func testCaseDownloadCsvHandler(w http.ResponseWriter, r *http.Request) {
	requestURL := r.URL.String()
	log.Printf("[REQ]\t%s\n", requestURL)

	testCases, err := ConvertExcelToJson()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	header := w.Header()
	header.Add("Content-Type", "text/csv")
	header.Add("Content-Disposition", "attachment; filename=\""+DefaultOutputFile[2:]+"\"")

	// CSVに吐き出す
	WriteCsv(w, testCases)
}

var (
	inputFile  string
	outputFile string
)

const (
	DefaultInputFile  = "./testcase.xlsx"
	DefaultOutputFile = "./testcase.csv"
)

func PrintUsage() {
	_, _ = fmt.Fprintf(os.Stderr, `
This tool uses port 10080 for internal web server API.
After running this tool, access "http://localhost:10080" with your favarite browser.

`)
}
func main() {

	// Parse arguments
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		PrintUsage()
		flag.PrintDefaults()
	}
	flag.CommandLine.Init(os.Args[0], flag.ExitOnError)

	flag.StringVar(&inputFile, "input", DefaultInputFile, "Specify file path of input [XLSX file].")
	flag.StringVar(&outputFile, "output", DefaultOutputFile, "Specify file path of output [CSV file].")
	flag.Parse()

	testCases, err := ConvertExcelToJson()
	if err == nil {
		// 出力先を生成(CSVファイル)
		outCsv, err := os.OpenFile(outputFile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer func() {
			_ = outCsv.Close()
		}()

		// CSVに吐き出す
		WriteCsv(outCsv, testCases)
	} else {
		// CSV変換失敗だがとりあえず起動はする
		log.Println(err)
	}

	http.HandleFunc("/ws", serveWs)
	http.HandleFunc("/api/testcases", testCaseDataHandler)
	http.HandleFunc("/api/downloadCsv", testCaseDownloadCsvHandler)
	http.HandleFunc("/", assetsHandler)

	flag.Usage()

	log.Fatal(http.ListenAndServe("localhost:10080", nil))
}

func ConvertExcelToJson() ([]*TestCase, error) {
	// 指定ファイルオープンしexcelize.Fileを取得
	xlsx, err := excelize.OpenFile(inputFile)
	if err != nil {
		return nil, err
	}

	var testCases []*TestCase

	testCases, err = BuildTestCasesFromXlsx(xlsx)
	if err != nil {
		return nil, err
	}
	return testCases, nil
}

func BuildTestCasesFromXlsx(xlsx *excelize.File) ([]*TestCase, error) {
	var testCases []*TestCase

	// 全シートを出力対象
	for _, sht := range xlsx.GetSheetMap() {
		rows := xlsx.GetRows(sht)

		// 1行目:ヘッダ
		headerRow := rows[0]
		// ヘッダ位置解析
		columnPositions := ColumnPositions{}
		err := columnPositions.AnalyseHeader(headerRow)
		if err != nil {
			return nil, err
		}
		//fmt.Println(columnPositions)

		// 2行目以降
		// テストケース解析
		for i := 1; i < len(rows); i++ {
			row := rows[i]

			// セクションはカラム番号若い順に入れていく
			var sections []string
			for _, value := range columnPositions.Sections {
				sections = append(sections, row[value])
			}

			newTestCase := NewTestCase(
				&row[columnPositions.ID],
				sections,
				&row[columnPositions.Title],
				&row[columnPositions.Type],
				&row[columnPositions.Priority],
				&row[columnPositions.Preconditions],
				&row[columnPositions.Steps],
				&row[columnPositions.ExpectedResult])
			valid, err := newTestCase.Validate()
			if valid {
				testCases = append(testCases, newTestCase)
			} else {
				err = errors.New(fmt.Sprintf("[Error] Line#: %d / %s\n", i+1, err.Error()))
				log.Println(err.Error())
				return nil, err
			}

			//fmt.Println(newTestCase)
		}
	}
	return testCases, nil
}

// Export csv
func WriteCsv(outCsv io.Writer, testCases []*TestCase) {
	csvWriter := csv.NewWriter(outCsv)
	defer csvWriter.Flush()
	// BOM (Excel で utf-8 CSVを開けるようにするためにBOMのUTF-8にする)
	_, _ = outCsv.Write([]byte{0xEF, 0xBB, 0xBF})
	// Header
	_ = csvWriter.Write(CsvHeaderArray)
	// Body
	for _, testCase := range testCases {
		_ = csvWriter.Write(testCase.ToArray())
	}
}

/**
  ファイ変更検知
*/
func NewWatcher() *fsnotify.Watcher {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = watcher.Add(inputFile)
	if err != nil {
		log.Fatal(err)
	}
	return watcher
}
