require "net/http"

class MediaMtxClient
  def initialize(base_url: ENV.fetch("MEDIA_MTX_API_URL", "http://mediamtx:9997"),
                 paths_endpoint: ENV.fetch("MEDIA_MTX_PATHS_ENDPOINT", "/v3/paths/list"))
    @base_url = base_url
    @paths_endpoint = paths_endpoint
  end

  def paths_by_name
    body = get_json(paths_endpoint)
    items = body.is_a?(Hash) ? body["items"] : nil
    return {} unless items.is_a?(Array)

    items.each_with_object({}) do |item, acc|
      next unless item.is_a?(Hash)

      name = item["name"].to_s
      next if name.empty?

      acc[name] = item
    end
  end

  private

  attr_reader :base_url, :paths_endpoint

  def get_json(endpoint)
    url = build_url(endpoint)
    req = Net::HTTP::Get.new(url)
    req["Accept"] = "application/json"

    Net::HTTP.start(url.host, url.port, use_ssl: url.scheme == "https", open_timeout: timeout_seconds,
                    read_timeout: timeout_seconds) do |http|
      response = http.request(req)
      unless response.is_a?(Net::HTTPSuccess)
        raise "mediamtx_api_http_#{response.code}"
      end

      JSON.parse(response.body)
    end
  end

  def build_url(endpoint)
    base = base_url.end_with?("/") ? base_url : "#{base_url}/"
    path = endpoint.to_s.sub(%r{\A/+}, "")
    URI.join(base, path)
  end

  def timeout_seconds
    ENV.fetch("MEDIA_MTX_API_TIMEOUT_SECONDS", "2").to_i
  end
end
