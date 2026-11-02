import { PublisherDashboardView } from '@/features/publisher/PublisherDashboardView';
import { PublisherGate } from '@/features/publisher/currentPublisher';

export default function PublisherPage() {
  return <PublisherGate>{(ctx) => <PublisherDashboardView ctx={ctx} />}</PublisherGate>;
}
