import logging
import sys

from thrift.protocol.TBinaryProtocol import TBinaryProtocolFactory
from thrift.protocol.TJSONProtocol import TJSONProtocolFactory
from thrift.protocol.TCompactProtocol import TCompactProtocolFactory
from thrift.Thrift import TApplicationException
from thrift.transport.TTransport import TTransportException
from frugal.protocol import FProtocolFactory
from frugal_test.f_FrugalTest import Numberz

PREAMBLE_HEADER = "preamble"
RAMBLE_HEADER = "ramble"

NATS_NAME = 'nats'
HTTP_NAME = 'http'


def get_nats_options():
    return {
        "verbose": True,
        "servers": ["nats://127.0.0.1:4222"]
        }


def get_protocol_factory(protocol):
    """
    Returns a protocol factory associated with the string protocol passed in
    as a command line argument to the cross runner

    :param protocol: string
    :return: Protocol factory
    """
    if protocol == "binary":
        return FProtocolFactory(TBinaryProtocolFactory())
    elif protocol == "compact":
        return FProtocolFactory(TCompactProtocolFactory())
    elif protocol == "json":
        return FProtocolFactory(TJSONProtocolFactory())
    else:
        logging.error("Unknown protocol type: %s", protocol)
        sys.exit(1)


def check_for_failure(actual, expected):
    """
    Compares the actual return results with the expected results.

    :return: Bool reflecting failure status

    """
    failed = False
    # TApplicationException doesn't implement __eq__ operator
    if isinstance(expected, TApplicationException):
        try:
            # Py2 and Py3 versions of thrift slightly differ in how the attribute is assigned...
            if sys.version_info[0] == 3 and actual.message.find(expected.message) == -1 or actual.type != expected.type:
                failed = True
            if sys.version_info[0] == 2 and actual._message.find(expected._message) == -1 or actual.type != expected.type:
                failed = True
        except Exception:
            failed = True
    elif isinstance(expected, TTransportException):
        if not isinstance(actual, TTransportException):
            failed = True
        elif actual.type != expected.type:
            failed = True
    elif isinstance(expected, Numberz):
        if type(actual) != type(expected):
            failed = True
    elif expected != actual:
        failed = True
    if failed:
        print(u"Unexpected result, expected:\n{e}\n but received:\n{a} ".format(
            e=expected, a=actual))

    return failed
