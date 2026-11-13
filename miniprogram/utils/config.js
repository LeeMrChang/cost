// DevTools development defaults to the local backend; every other runtime uses HTTPS.
// Saved debug overrides apply only inside DevTools, never to phones or release builds.
// Never place AppSecret, database passwords or session tokens in this public file.
module.exports = {
 baseURL: 'https://api.xiachen.xyz',
 developmentBaseURL: 'http://127.0.0.1:8081',
};
