// Shared by both api.ts (client-side fetch) and api.server.ts (server
// component fetch) so the two independent fetch layers can't drift into
// having subtly different ApiError shapes despite `instanceof ApiError`
// checks being used across that boundary (e.g. FormCus, a server
// component, checking an error thrown by api.server.ts).
export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}
