import { Code, ConnectError } from '@connectrpc/connect';

export async function callAdminRpc<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call();
  } catch (error) {
    const connectError = ConnectError.from(error);
    if (connectError.code === Code.Canceled) {
      throw connectError;
    }
    if (connectError.code === Code.Unknown && connectError.cause instanceof TypeError) {
      throw new Error('Unable to reach the admin API', { cause: connectError });
    }
    throw new Error(connectError.rawMessage || 'Admin API request failed', { cause: connectError });
  }
}
