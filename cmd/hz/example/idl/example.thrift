namespace go cloudwego.hertz.template

struct Req {
  1: optional string QueryString(api.query="q1");
  2: optional string PathString(api.path="p1");
  3: optional string HeaderString(api.header="h1");
  4: optional string MixString(api.query="q2", api.header="h2", api.path="p2");
  5: optional string BodyString(api.body="sad");
}

struct Resp {
  1: optional string RespString(api.header="h1");
}

service Hertz {
    // 以下为一个组，并且分别使用了不同类型的request
    Resp BizMethod2(1: Req req)(api.post = '/life/client1');
    Resp BizMethod3(1: Req req)(api.post = '/life/client2');
    Resp BizMethod4(1: Req req)(api.post = '/life/client3');
}