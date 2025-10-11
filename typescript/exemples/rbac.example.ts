// apps/web/src/client/examples/rbac.getAccounts.example.ts
import { globular } from '../index';

// Import the generated client + messages from your published package.
// Adjust the path if your package structure differs.
import { ResourceServiceClient } from '../resource/resource_grpc_web_pb';
import * as resource from '../resource/resource_pb';

export async function streamAccounts() {
  // Ensure the generic is provided so TS doesn't infer `unknown`
  const client = globular.client<ResourceServiceClient>('resource.ResourceService', ResourceServiceClient);

  // Build auth/tenant metadata via your helper (cookies, token, etc.)
  const md = await globular.metadata();

  // Request can be empty unless your service requires filters/paging
  const rq = new resource.GetAccountsRqst();
  // Example: if your proto defines fields like page_size or filter:
  // rq.setPageSize(100);
  // rq.setFilter('active');

  // GetAccounts is server-streaming => returns a ClientReadableStream<GetAccountsRsp>
  const stream = client.getAccounts(rq, md);

  return new Promise<void>((resolve, reject) => {
    stream.on('data', (msg: resource.GetAccountsRsp) => {
      // Depending on your proto, access fields via getters or toObject()
      // If the response contains a repeated 'accounts' field:
      // const accounts = msg.getAccountsList();
      // console.log('accounts chunk:', accounts);
      console.log('accounts message:', msg.toObject ? msg.toObject() : msg);
    });
    stream.on('end', () => resolve());
    stream.on('error', (e: any) => reject(e));
    // (optional) stream.on('status', (s) => console.log('status:', s));
  });
}