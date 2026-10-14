export type KV = { key: string; value: string; enabled: boolean; secret?: boolean };

export type Auth = {
  type: 'none' | 'bearer' | 'basic' | 'apikey';
  token: string;
  username: string;
  password: string;
  key: string;
  value: string;
  in: 'header' | 'query';
};

export type Body = {
  // In graphql mode raw is the query and variables a JSON object.
  mode: 'none' | 'raw' | 'urlencoded' | 'form' | 'graphql';
  raw: string;
  contentType: string;
  form: KV[];
  variables?: string;
};

// For gRPC, url is the server address, method is package.Service/Method,
// headers are metadata and body.raw is the message as JSON.
export type Assertion = { target: string; op: 'eq' | 'ne' | 'contains' | 'exists' | 'lt' | 'gt' | 'matches'; value?: string };
export type Extract = { var: string; from: string };
// op is "extract" for a variable that could not be read from the response.
export type TestResult = { target: string; op: Assertion['op'] | 'extract'; value?: string; passed: boolean; message?: string };

export type Req = {
  protocol?: 'grpc';
  method: string;
  url: string;
  headers: KV[];
  auth: Auth;
  body: Body;
  tests: Assertion[];
  extract: Extract[];
  docs?: string;
};

export type Kind = 'http' | 'graphql' | 'grpc';

// An item without a request is a folder.
export type Item = { id: string; name: string; request?: Req; items?: Item[] };
// Local .proto files for the collection's gRPC requests. roots are import directories (like protoc -I),
// files are relative to a root. With neither, gRPC uses server reflection.
export type ProtoConfig = { roots?: string[]; files?: string[] };
export type ProtoCheck = { files: number; services: string[]; methods: string[] };
export type Collection = { id: string; name: string; items: Item[]; proto?: ProtoConfig };
export type Environment = { id: string; name: string; variables: KV[] };

export type HistoryEntry = { id: string; at: string; request: Req; itemId?: string; status: number; timeMs: number; error?: string; passed?: number; failed?: number };

export type Res = {
  status: number;
  statusText: string;
  headers: { key: string; value: string }[];
  body: string;
  base64: boolean;
  size: number;
  timeMs: number;
  error?: string;
  // status is a gRPC code (0 = OK) instead of an HTTP status
  grpc?: boolean;
  tests?: TestResult[];
  // a response from history whose body was too large to keep
  bodyNotKept?: boolean;
};

export type GitChange = { path: string; status: 'added' | 'modified' | 'deleted' };
export type GitStatus = { repo: boolean; remote: string; branch: string; upstream: boolean; ahead: number; behind: number; changes: GitChange[] };
// recent: other workspace folders opened on this machine, newest first.
export type Workspace = { path: string; gitAvailable: boolean; git: GitStatus; recent: string[] };

// A gRPC method: its message types and an example request message as JSON.
export type GrpcMethodInfo = { input: string; output: string; streaming: boolean; template: string };

// One directory, as listed for the folder and file browser.
export type FsEntry = { name: string; dir: boolean };
export type FsListing = { path: string; parent: string; entries: FsEntry[]; home: string; workspace: string };
