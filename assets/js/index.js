let app = new Vue({
  el: '#app',
  data: {
    rowsPerPageItems: [
      1,
      {
        text: 'ALL',
        value: -1
      }
    ],
    pagination: {
      rowsPerPage: 1
    },
    items: [],
    saneConverter: null,
    loading: true,
    snackbar: false,
    snackbarText: "",
    snackbarTimeout: 1000,
  },

  created: function () {
    let self = this;

    // self.saneConverter = Markdown.getSanitizingConverter();
    self.saneConverter = new Markdown.Converter();

    // テーブルをなんとなくサポートするHook処理を登録
    self.saneConverter.hooks.chain("preBlockGamut", function (text) {

      return text.replace(/((^\|\|.+\n(^\|\|.+\n)*\n*)+)/gm,
        function (wholeMatch, m1) {
          // console.log("wholeMatch:" + wholeMatch);
          // console.log("m1:" + m1);

          let splits = m1.split("\n");
          let table = "<table>";
          for (let i = 0; i < splits.length; i++) {
            let line = splits[i];
            if (/^\|\|\|.*$/g.test(line)) {
              // ヘッダ行
              let inners = line.replace("|||", "").split("|");
              let str = "<tr>";
              for (let r in inners) {
                str += "<th>" + inners[r].trim() + "</th>";
              }
              str += "</tr>";
              table += str;
            } else if (!/^$/.test(line)) {
              // ヘッダ行ではない
              let inners = line.replace(/^(\|\|)(.+)/, "$2").split("|");
              let str = "<tr>";
              for (let r in inners) {
                str += "<td>" + inners[r].trim() + "</td>";
              }
              str += "</tr>";
              table += str;
            }
          }
          table += "</table>\n";

          return table;
        }
      );
    });

    axios.get('/api/testcases')
      .then(function (response) {
        self.items = response.data;
        self.loading = false;
      })
      .catch(reason => {
        console.error(reason);
        self.loading = false;
        self.snackbar = true;
        self.snackbarText = "Error: Input file maybe contain some invalid content";
        self.snackbarTimeout = 10 * 1000;
      });

    // WS
    var conn = new WebSocket("ws://localhost:10080/ws");
    conn.onclose = function (evt) {
      console.log('Connection closed');
    };
    conn.onmessage = function (evt) {
      let data = JSON.parse(evt.data);
      if (data.err) {
        console.error(data.err);
        self.snackbarText = "Error: " + data.err;
        self.snackbarTimeout = 10 * 1000;
      } else {
        self.items = data.testCases;
        self.snackbarText = "Updated.";
        self.snackbarTimeout = 1000;
      }
      self.snackbar = true;
    };

    // key
    document.addEventListener("keydown", function (e) {
      switch (e.key) {
        case "ArrowLeft":
          self.prevPage();
          break;
        case "ArrowRight":
          self.nextPage();
          break;
      }
    });

  },

  methods: {
    convertMarkdownToHtml: function (md) {
      return this.saneConverter.makeHtml(md);
    },
    nextPage: function () {
      if (this.pagination.rowsPerPage < 0 ||
        this.pagination.page * this.pagination.rowsPerPage >= this.$refs["dataiterator"].itemsLength ||
        this.$refs["dataiterator"].pageStop < 0) {
        return;
      }
      const cp = this.$refs["dataiterator"].computedPagination.page;
      this.$refs["dataiterator"].updatePagination({page: cp + 1,})
    },
    prevPage: function () {
      const cp = this.$refs["dataiterator"].computedPagination.page;
      if (cp === 1) {
        return;
      }
      this.$refs["dataiterator"].updatePagination({page: cp - 1,})
    }
  },
});
