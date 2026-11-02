import { useParams } from 'react-router-dom';
import { PackageDetailsView } from '@/features/package-details/PackageDetailsView';

export default function PackagePage() {
  const { publisher = '', slug = '' } = useParams();
  return <PackageDetailsView key={`${publisher}/${slug}`} publisher={publisher} slug={slug} />;
}
