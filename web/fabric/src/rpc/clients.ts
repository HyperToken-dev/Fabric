import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { ApiKeyAdminService } from '../gen/api_key_admin_pb';
import { ApiKeyClientService } from '../gen/api_key_client_pb';
import { AuthService } from '../gen/auth_pb';
import { ChannelAdminService } from '../gen/channel_admin_pb';
import { ChannelClientService } from '../gen/channel_client_pb';
import { ModelAdminService } from '../gen/model_admin_pb';
import { ModelClientService } from '../gen/model_client_pb';
import { UsageService } from '../gen/usage_pb';
import { IntegralLogService } from '../gen/integral_pb';
import { SensitiveWordService } from '../gen/sensitive_pb';

const transport = createConnectTransport({ baseUrl: '/admin-api' });

export const authClient = createClient(AuthService, transport);
export const apiKeyAdminClient = createClient(ApiKeyAdminService, transport);
export const apiKeyClient = createClient(ApiKeyClientService, transport);
export const channelAdminClient = createClient(ChannelAdminService, transport);
export const channelClient = createClient(ChannelClientService, transport);
export const modelAdminClient = createClient(ModelAdminService, transport);
export const modelClient = createClient(ModelClientService, transport);
export const usageClient = createClient(UsageService, transport);
export const integralLogClient = createClient(IntegralLogService, transport);
export const sensitiveWordClient = createClient(SensitiveWordService, transport);
