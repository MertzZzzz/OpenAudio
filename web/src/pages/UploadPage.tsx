import { UploadWizardView } from '@/features/upload/UploadWizardView';
import { PublisherGate } from '@/features/publisher/currentPublisher';

export default function UploadPage() {
  return <PublisherGate>{(ctx) => <UploadWizardView ctx={ctx} />}</PublisherGate>;
}
