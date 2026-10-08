// Bootstrap do Swagger UI servido localmente (F2.1 do roadmap de
// conformidade) — assets vendorados de swagger-ui-dist@5.17.14, para que
// /docs não dependa de um CDN externo e a CSP possa manter script-src
// 'self' sem allowlist de host.
window.onload = function () {
  window.ui = SwaggerUIBundle({
    url: "/openapi.json",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis],
    layout: "BaseLayout",
  });
};
