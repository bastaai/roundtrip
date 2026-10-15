// What each part of a request is, shown when the pointer rests on its label.

export const HINTS = {
  params: 'Query parameters, added to the URL after the ?. Editing them here edits the URL.',
  headers: 'HTTP headers sent with the request. Untick a row to leave it out without deleting it.',
  metadata: 'gRPC metadata sent with the call, the gRPC equivalent of HTTP headers.',
  auth: 'Credentials added to the request when it is sent. Values can use {{variables}}.',
  body: 'What the request sends after its headers.',
  query: 'The GraphQL query or mutation, sent as JSON together with its variables.',
  message: 'The request message as JSON, using the proto JSON mapping.',
  tests: 'Checks run on the response, and values saved from it as variables for later requests.',
  docs: "Notes about this request, saved in the request's file.",

  headerKey: 'The header name, e.g. Content-Type. Case does not matter.',
  metadataKey: 'The metadata key, lower case, e.g. x-request-id.',
  headerValue: 'The value. {{variables}} are filled in from the environment when sent.',
  paramKey: 'The parameter name, e.g. page.',
  paramValue: 'The parameter value, URL-encoded when sent.',
  formKey: 'The form field name.',
  formValue: 'The form field value.',

  authType: 'How the request proves who it is.',
  authNone: 'Nothing is added.',
  bearer: 'Sends "Authorization: Bearer <token>", the usual form for OAuth and JWT tokens.',
  basic: 'Sends "Authorization: Basic" with the username and password base64-encoded.',
  apikey: 'Sends a key and value of your choice as a header or query parameter.',
  token: 'The token, without the "Bearer " prefix. Use a {{variable}} so it stays out of git.',
  username: 'The username for Basic auth.',
  password: 'The password for Basic auth. Use a {{variable}} so it stays out of git.',
  apiKeyName: 'The header or query parameter name the server expects, e.g. X-API-Key.',
  apiKeyValue: 'The key itself. Use a {{variable}} so it stays out of git.',
  apiKeyIn: 'Whether the key goes in a header or in the URL query.',

  bodyNone: 'Send no body.',
  bodyRaw: 'Send the text as written, with the chosen Content-Type.',
  bodyUrlencoded: 'Send fields as application/x-www-form-urlencoded, like an HTML form.',
  bodyForm: 'Send fields as multipart/form-data.',
  contentType: 'The Content-Type header sent with a raw body, unless a header sets it.',
  gqlQuery: 'The operation to run. Declare variables like ($id: ID!) and use them as $id.',
  gqlVariables: 'A JSON object with values for the variables the query declares.',

  target:
    'What to check: status, time (ms), header.<name>, body, or a JSON path into the body such as body.user.id or body.items[0].name.',
  check: 'How the target is compared to the expected value.',
  expected: 'The value to compare with. {{variables}} are filled in.',
  extractVar: 'The variable name. Later requests use it as {{name}}.',
  extractFrom: 'Where the value comes from: status, header.<name>, or a JSON path such as body.access_token.',
} as const;

export const OP_HINTS: Record<string, string> = {
  eq: 'The target is exactly the expected value.',
  ne: 'The target is anything but the expected value.',
  contains: 'The expected value appears somewhere in the target.',
  exists: 'The target is present in the response, whatever its value.',
  lt: 'The target is a number below the expected value.',
  gt: 'The target is a number above the expected value.',
  matches: 'The target matches the expected value as a regular expression.',
};
