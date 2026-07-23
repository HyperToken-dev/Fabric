import { createClient } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { ManageApiKeyService } from '../gen/apiKey_pb';
import { ChannelService } from '../gen/channel_pb';
import { ModelService } from '../gen/model_pb';
import { UsageService } from '../gen/usage_pb';
import { IntegralLogService } from '../gen/integral_pb';

const transport = createConnectTransport({ baseUrl: '/admin-api' });

export const apiKeyClient = createClient(ManageApiKeyService, transport);
export const channelClient = createClient(ChannelService, transport);
export const modelClient = createClient(ModelService, transport);
export const usageClient = createClient(UsageService, transport);
export const integralLogClient = createClient(IntegralLogService, transport);
