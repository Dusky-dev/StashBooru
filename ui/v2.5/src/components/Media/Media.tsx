import { Helmet } from "react-helmet";
import { Redirect, useLocation } from "react-router-dom";
import { useTitleProps } from "src/hooks/title";
import { FilteredMediaList } from "./MediaList";

export default function Media() {
  const location = useLocation();
  const titleProps = useTitleProps({ id: "all" });
  const legacyDestination =
    location.pathname === "/media/images"
      ? "/images"
      : location.pathname === "/media/videos"
        ? "/scenes"
        : undefined;

  if (legacyDestination) {
    return <Redirect to={{ ...location, pathname: legacyDestination }} />;
  }

  return (
    <>
      <Helmet {...titleProps} />
      <FilteredMediaList />
    </>
  );
}
