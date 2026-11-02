import { Link } from 'react-router-dom';
import { Compass } from 'lucide-react';
import { Button } from '@/shared/ui/Button';
import { EmptyState } from '@/shared/ui/EmptyState';

export default function NotFoundPage() {
  return <EmptyState icon={<Compass className="h-6 w-6" />} title="Page not found" description="The page you’re looking for doesn’t exist." action={<Link to="/"><Button>Go to Discover</Button></Link>} />;
}
